package crawler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	sitemap "github.com/oxffaa/gopher-parse-sitemap"
	"github.com/temoto/robotstxt"
	"go.uber.org/zap"
)

type Spider struct {
	job        Job
	queue      *ProcessingQueue
	log        *zap.Logger
	ctx        context.Context
	robotsChan chan<- RobotsChanInfo

	client       *http.Client
	robots       *SafeMap[string, robotstxt.RobotsData]
	crawledSites *SafeMap[string, SiteMetadata]
}

func NewSpider(ctx context.Context, logger *zap.Logger, job Job, queue *ProcessingQueue, robots *SafeMap[string, robotstxt.RobotsData], robotsChan chan<- RobotsChanInfo, crawledSites *SafeMap[string, SiteMetadata]) *Spider {
	return &Spider{
		queue:        queue,
		log:          logger,
		ctx:          ctx,
		robots:       robots,
		client:       getHttpClient(),
		job:          job,
		robotsChan:   robotsChan,
		crawledSites: crawledSites,
	}
}

func (sp *Spider) Run() *SiteMetadata {
	client := getHttpClient()
	u, err := url.Parse(sp.job.url)

	if err != nil {
		sp.log.Error("error with seed url given %s", zap.Error(err))
		return nil
	}

	if sp.crawledSites.Contains(sp.job.url) {
		return nil
	}
	hostname := u.Hostname()
	sp.processRobots()
	//sp.parseSitemap(sp.job.url)

	if !sp.shouldCrawl(sp.job.url) {
		return nil
	}

	res, err := client.Get(sp.job.url)
	if err != nil {
		if !strings.Contains(err.Error(), "certificate") {
			sp.log.Error("fetching seed (%s) not working",
				zap.String("seed", sp.job.url),
				zap.Error(err),
				zap.Bool("shouldCrawl", sp.shouldCrawl(sp.job.url)))

		}
		return nil
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		if res.StatusCode == 429 {
			sp.log.Debug("Rate limited")
		}
		return nil
	}

	foundUrls := sp.findLinks(res, hostname)

	//sp.log.Debug("Finished Processing", zap.String("url", sp.job.url))

	for _, url := range foundUrls {
		go sp.queue.Push(Job{url, sp.job.depth + 1})
	}

	return &SiteMetadata{
		URL: sp.job.url,
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
			href = sp.job.url + href
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
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 90,
		IdleConnTimeout:     80 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second, 
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,

		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
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

	avoidedPhrases := []string{
		"account",
		"account",
		"auth",
		"login",
	}

	for _, phrase := range avoidedPhrases {
		if strings.Contains(host, phrase) {
			return false
		}
	}

	// Extension check
	extPattern := regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|css|js|pdf|zip)(\?|$)`)
	if extPattern.MatchString(u.Path) {
		return false
	}

	// Path-segment check
	pathPattern := regexp.MustCompile(`(?i)/(login|cart|admin)(/|$)`)
	if pathPattern.MatchString(u.Path) {
		return false
	}

	return true
}

func (sp *Spider) processRobots() {
	url, err := url.Parse(sp.job.url)
	if err != nil {
		sp.log.Error("Invalid url (robots.txt)", zap.String("url", sp.job.url))
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

func (sp *Spider) parseSitemap(site string) error {
	err := sitemap.ParseIndexFromSite(site+"/sitemap.xml", func(ie sitemap.IndexEntry) error {
		sp.log.Debug("Found sitemap child", zap.String("url", ie.GetLocation()))
		child := ie.GetLocation()
		return sp.parseSitemap(child)

	})

	if err == nil {
		//indices shouldn't have their own links to search
		return nil
	}

	err = sitemap.ParseFromSite(site+"/sitemap.xml", func(e sitemap.Entry) error {
		sp.log.Debug("Added from sitemap", zap.String("url", e.GetLocation()))
		sp.queue.Push(Job{
			url: e.GetLocation(),
		})

		return nil
	})

	if err != nil {
		//might get situations where the sitemap is just a not found page
		//sp.log.Error("Sitemap parsing error", zap.Error(err))
		return nil
	}

	return nil
}
