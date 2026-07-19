package crawler

import (
	"net"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

type BQueue struct {
	Host        string
	URLs        []string
	Locked      bool
	LockedUntil time.Time

	mu sync.Mutex
}

type SpiderPayload struct {
	url  string
	host string
	ip   net.IP
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

	log *zap.Logger
	mu  sync.Mutex
}

func NewFrontierStore(log *zap.Logger) *FrontierStore {
	return &FrontierStore{
		bufferQueues: NewSafeMap[string, *BQueue](),
		readyQueues:  NewSafeMap[int, chan SpiderPayload](),
		ips:          NewSafeMap[string, net.IP](),
		receiveChan:  make(chan PageMetadata),
		log:          log,
	}
}

func (fs *FrontierStore) Run() {
	const workers = 10
	var wg sync.WaitGroup

	pagesCrawled := 0
	pageReceiveChan := make(chan PageMetadata, 128)
	linkReceiveChan := make(chan string, 1024)

	wg.Add(1)
	fs.log.Info("Seeding Frontier")

	go func() {
		for _, link := range seed {
			fs.AddUrl(link, linkReceiveChan)
		}
	}()

	go func() {
		for w := 1; w <= workers; w++ {
			jobs := make(chan SpiderPayload, 1)
			fs.readyQueues.Set(w, jobs)
			spider := NewSpider(w, fs.log, jobs, pageReceiveChan)

			go spider.Walk()
		}
	}()

	go func() {
		for {
			select {
			case meta := <-pageReceiveChan:
				fs.log.Info("Page metadata received", zap.String("url", meta.URL))
				bQueue, ok := fs.bufferQueues.Get(meta.Host)
				pagesCrawled++

				if !ok {
          
					fs.log.Warn("Buffer queue should exist but doesn't")
          pageReceiveChan <- meta
          break
				}
        
        bQueue.mu.Lock()
				bQueue.Locked = false
        bQueue.mu.Unlock()

				for _, link := range meta.Links {
					fs.AddUrl(link, linkReceiveChan)
				}

			case link := <-linkReceiveChan:
				url, err := url.Parse(link)

				if err != nil {
					fs.log.Warn("Invalid url given (linkReceiveChan)", zap.String("url", link))
					return
				}

				hostname := url.Hostname()

				bQueue, ok := fs.bufferQueues.Get(hostname)
				bQueue.mu.Lock()

				if !ok {
					bQueue.mu.Unlock()
					continue //might cause issues later
				}

				if bQueue.Locked {
					fs.log.Debug("LOCKED Adding to buffer", zap.String("host", hostname), zap.String("url", link))
					bQueue.URLs = append(bQueue.URLs, hostname)
					bQueue.mu.Unlock()
					break
				}

				addedToNone := true
				//TODO make it an actual queue
				fs.readyQueues.Range(func(id int, sendChan chan SpiderPayload) bool {
					if len(sendChan) == 0 {
						sendChan <- SpiderPayload{
							url: link,
              host: hostname,
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
					bQueue.URLs = append(bQueue.URLs, hostname)
				}

				bQueue.mu.Unlock()
			}
		}
	}()

	go func() {
		for {
			if pagesCrawled >= 10 {
				wg.Done()
				return
			}
		}
	}()

	wg.Wait()

	for w := 1; w <= workers; w++ {
		c, _ := fs.readyQueues.Get(w)
		close(c)
	}

	close(pageReceiveChan) //might be a code smell
  fs.log.Info("Finished Crawling", zap.Int("count", pagesCrawled))
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
		ips, err := net.LookupIP(hostname)
    
		if err != nil {
      fs.log.Warn("Could not find ip", zap.String("host", hostname))
			return //must be something wrong with hostname
		}
    
		fs.ips.Set(hostname, ips[0])
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
      fs.log.Debug("Adding url to prexisting BQueue")

    }
	linkReceiveChan <- rawUrl
}
