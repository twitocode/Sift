package crawler

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type BQueue struct {
	Host        string
	URLs        []string
	Locked      bool
	LockedUntil time.Time

	mu sync.Mutex
}

type DialerContext func(ctx context.Context, network, addr string) (net.Conn, error)
type SpiderPayload struct {
	url           string
	host          string
	dialerContext DialerContext
}

type PageMetadata struct {
	URL         string
	Host        string
	Title       string
	Description string
	Text        string
	Links       []string
	StatusCode  int // some pages return 429 stuff like that so i can filter out later if needed
	CrawledAt   time.Time
	ContentHash string //TODO: duplication detection, hash text form page (different urls same text)
}

type FrontierStore struct {
	bufferQueues *SafeMap[string, *BQueue]
	readyQueues  *SafeMap[int, chan SpiderPayload]

	ips         *SafeMap[string, net.IP]
	receiveChan chan PageMetadata
	dnsGroup    singleflight.Group

	crawled *SafeMap[string, *PageMetadata]

	log *zap.Logger
	mu  sync.Mutex
}

func NewFrontierStore(log *zap.Logger) *FrontierStore {
	return &FrontierStore{
		bufferQueues: NewSafeMap[string, *BQueue](),
		readyQueues:  NewSafeMap[int, chan SpiderPayload](),
		ips:          NewSafeMap[string, net.IP](),
		receiveChan:  make(chan PageMetadata),
		crawled:      NewSafeMap[string, *PageMetadata](),
		log:          log,
	}
}

func (fs *FrontierStore) Run() {
	const workers = 10
	var wg sync.WaitGroup

	startTime := time.Now()
	pagesCrawled := 0
	pageReceiveChan := make(chan *PageMetadata, 128)
	linkReceiveChan := make(chan string, 1024)

	wg.Add(1)
	fs.log.Info("Seeding Frontier")

	go func() {
		for _, link := range seed {
			go fs.AddUrl(link, linkReceiveChan)
		}
	}()

	go func() {
		for w := 1; w <= workers; w++ {
			jobs := make(chan SpiderPayload, 1)
			fs.readyQueues.Set(w, jobs)
			spider := NewSpider(w, fs.log, jobs, pageReceiveChan, fs.dialContext)

			go spider.Walk()
		}
	}()

	go func() {
		for {
			select {
			case meta := <-pageReceiveChan:
				//might not need this
				if fs.HasLinkBeenCrawled(meta.URL) {
					fs.log.Info("Duplicate link crawled", zap.String("url", meta.URL))
					continue
				}
				fs.log.Info("Page metadata received", zap.String("url", meta.URL))

				fs.crawled.Set(meta.URL, meta)
				if pagesCrawled >= 10 {
					wg.Done()
					return
				}

				pagesCrawled++

				bQueue, ok := fs.bufferQueues.Get(meta.Host)
				if !ok {
					fs.log.Warn("Buffer queue should exist but doesn't")
					pageReceiveChan <- meta
					break
				}

				bQueue.mu.Lock()
				bQueue.Locked = false
				bQueue.mu.Unlock()

				for _, link := range meta.Links {
					if fs.HasLinkBeenCrawled(link) {
						fs.log.Info("Duplicate link crawled", zap.String("url", meta.URL))
						continue
					}
					go fs.AddUrl(link, linkReceiveChan)
				}

			case link := <-linkReceiveChan:
				url, err := url.Parse(link)

				if err != nil {
					fs.log.Warn("Invalid url given (linkReceiveChan)", zap.String("url", link))
					return
				}

				hostname := url.Hostname()
				bQueue, ok := fs.bufferQueues.Get(hostname)

				if !ok {
					continue //might cause issues later
				}

				bQueue.mu.Lock()

				if bQueue.Locked {
					fs.log.Debug("LOCKED Adding to buffer", zap.String("host", hostname), zap.String("url", link))
					bQueue.URLs = append(bQueue.URLs, link)
					bQueue.mu.Unlock()
					break
				}

				addedToNone := true
				//TODO make it an actual queue
				fs.readyQueues.Range(func(id int, sendChan chan SpiderPayload) bool {
					if len(sendChan) == 0 {
						sendChan <- SpiderPayload{
							url:           link,
							host:          hostname,
							dialerContext: fs.dialContext,
						}

						bQueue.Locked = true
						addedToNone = false
						fs.log.Debug("Found empty ready queue")
						return false
					}

					return true
				})

				if addedToNone {
					fs.log.Debug("Adding back to buffer", zap.String("host", hostname), zap.String("url", link))
					bQueue.URLs = append(bQueue.URLs, link)
				}

				bQueue.mu.Unlock()
			}
		}
	}()

	wg.Wait()

	for w := 1; w <= workers; w++ {
		c, _ := fs.readyQueues.Get(w)
		close(c)
	}

	close(pageReceiveChan) //might be a code smell
	fs.log.Info("Finished Crawling", zap.Int("count", pagesCrawled), zap.Duration("elapsed", time.Since(startTime)))

	fs.crawled.Range(func(k string, v *PageMetadata) bool {
		fmt.Printf("%s\n", k)
		return true
	})
}

func (fs *FrontierStore) AddUrl(rawUrl string, linkReceiveChan chan<- string) {
	url, err := url.Parse(rawUrl)

	if err != nil {
		fs.log.Warn("Invalid url given", zap.String("url", rawUrl))
		return
	}

	hostname := url.Hostname()

	if !fs.ips.Contains(hostname) {
		fs.log.Debug("IP cache miss", zap.String("hostname", hostname))

		v, err, shared := fs.dnsGroup.Do(hostname, func() (interface{}, error) {
			return net.LookupIP(hostname)
		})

		if err != nil {
			fs.log.Warn("Could not find ip", zap.String("host", hostname))
			return //must be something wrong with hostname
		}

		ips := v.([]net.IP)
		fs.ips.Set(hostname, ips[0])

		if shared {
			fs.log.Debug("Suppressed duplicate concurrent DNS lookups", zap.String("hostname", hostname))
		}
	}

	queue, ok := fs.bufferQueues.Get(hostname)
	if !ok {
		fs.log.Debug("BQueue cache miss", zap.String("host", hostname))
		queue = &BQueue{
			Host:        hostname,
			URLs:        []string{},
			Locked:      false,
			LockedUntil: time.Now(),
		}

		fs.bufferQueues.Set(hostname, queue)
	} else {
		queue.mu.Lock()
		queue.URLs = append(queue.URLs, rawUrl)
		queue.mu.Unlock()
		fs.log.Debug("Adding url to prexisting BQueue", zap.String("url", rawUrl))
	}

	linkReceiveChan <- rawUrl
}

func (fs *FrontierStore) HasLinkBeenCrawled(link string) bool {
	//TODO: use bloom filters and hashing instead
	return fs.crawled.Contains(link)
}

func (fs *FrontierStore) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}

	ip, ok := fs.ips.Get(host)
	if !ok {
		return dialer.DialContext(ctx, network, addr)
	}

	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}
