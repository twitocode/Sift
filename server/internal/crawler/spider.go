package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt"
	"go.uber.org/zap"
)

var EXCLUDED_REGEX = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|css|js|pdf|zip)$|(login|cart|admin)`)

var MAX_DEPTH = 3

type Spider struct {
	seed       string
	item       Item
	queue      *ProcessingQueue
	log        *zap.Logger
	ctx        context.Context
	robotsChan chan<- RobotsChanInfo

	client *http.Client
	robots *SafeMap[string, robotstxt.RobotsData]
}

func NewSpider(ctx context.Context, logger *zap.Logger, item Item, queue *ProcessingQueue, robots *SafeMap[string, robotstxt.RobotsData], robotsChan chan<- RobotsChanInfo) *Spider {
	return &Spider{
		seed:       item.url,
		queue:      queue,
		log:        logger,
		ctx:        ctx,
		robots:     robots,
		client:     getHttpClient(),
		item:       item,
		robotsChan: robotsChan,
	}
}

func (sp *Spider) Run() {
	client := getHttpClient()
	u, err := url.Parse(sp.seed)
	if err != nil {
		sp.log.Error("error with seed url given %s", zap.Error(err))
		return
	}

	hostname := u.Hostname()
	sp.processRobots()

	if !sp.shouldCrawl(sp.seed) {
		return
	}

	res, err := client.Get(sp.seed)
	if err != nil {

		sp.log.Error("fetching seed (%s) not working",
			zap.String("seed", sp.seed),
			zap.Error(err),
			zap.Bool("shouldCrawl", sp.shouldCrawl(sp.seed)))
		return
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		if res.StatusCode == 429 {
			sp.log.Debug("Rate limited")
		}
		return
	}

	foundUrls := sp.findLinks(res, hostname)

	sp.log.Debug("Finished Processing", zap.String("url", sp.seed))

	for _, url := range foundUrls {
		sp.queue.Push(Item{url})
	}
}

func (sp *Spider) findLinks(res *http.Response, hostname string) []string {
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		sp.log.Error("goquery error", zap.Error(err))
		return nil
	}

	var foundUrls []string = make([]string, 0)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")

		if !sp.shouldCrawl(href) {
			return
		}
		parsedHref, err := url.Parse(href)

		if err != nil {
			return
		}

		//remove query strings and headers
		href = strings.Split(href, "#")[0]
		href = strings.Split(href, "?")[0]

		if !parsedHref.IsAbs() {
			href = sp.seed + href
		}

		allowed := true
		group, ok := sp.robots.Get(hostname)
		if ok {
			allowed = group.FindGroup("*").Test(href)
		}

		if allowed && exists && (parsedHref.Scheme == "http" || parsedHref.Scheme == "https") {
			foundUrls = append(foundUrls, href)
		}
	})

	slices.Sort(foundUrls)
	foundUrls = slices.Compact(foundUrls)
	return foundUrls
}

func getHttpClient() *http.Client {
	transport := &http.Transport{}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 1 redirects")
			}
			return nil
		},
	}
}

func (sp *Spider) shouldCrawl(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())

	// Block any host that starts with "login." — auth subdomains, not content
	if strings.HasPrefix(host, "login.") || strings.HasPrefix(host, "auth.") || strings.HasPrefix(host, "accounts.") || strings.HasPrefix(host, "app.") {
		return false
	}

	// Extension check — only on the path
	extPattern := regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|css|js|pdf|zip)(\?|$)`)
	if extPattern.MatchString(u.Path) {
		return false
	}

	// Path-segment check — only on the path, not the host
	pathPattern := regexp.MustCompile(`(?i)/(login|cart|admin)(/|$)`)
	if pathPattern.MatchString(u.Path) {
		return false
	}

	return true
}

func (sp *Spider) processRobots() {
	url, err := url.Parse(sp.item.url)
	if err != nil {
		sp.log.Error("Invalid url (robots.txt)", zap.String("url", sp.item.url))
	}
	hostname := url.Hostname()

	if sp.robots.Contains(hostname) {
		//sp.log.Debug("Robots already added", zap.String("hostname", hostname))
		return
	}

	res, err := sp.client.Get(fmt.Sprintf("https://%s/robots.txt", hostname))

	if err != nil { // might not have robots.txt
		sp.log.Error("Error fetching robots.txt", zap.Error(err))
	} else {
		defer res.Body.Close()
		robots, err := robotstxt.FromResponse(res)

		if err != nil {
			sp.log.Error("Error parsing robots.txt", zap.Error(err))
			return
		}

		sp.robotsChan <- RobotsChanInfo{
			name:   hostname,
			robots: *robots,
		}
	}
}
