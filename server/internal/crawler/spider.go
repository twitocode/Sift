package crawler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt"
)

var EXCLUDED_REGEX = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|css|js|pdf|zip)$|(login|cart|admin)`)

var MAX_DEPTH = 3

type Spider struct {
	siteRobotsMap map[string]*robotstxt.Group
	seed          string
	graph         SiteGraph
	linkChan      chan<- SiteAdjacency
}

func NewSpider(seed string, graph SiteGraph, linkChan chan<- SiteAdjacency) *Spider {
	return &Spider{
		siteRobotsMap: make(map[string]*robotstxt.Group),
		seed:          seed,
		graph:         graph,
		linkChan:      linkChan,
	}
}

func (sp *Spider) Run() {
	client := getHttpClient()
	u, err := url.Parse(sp.seed)
	if err != nil {
		log.Println("error with seed url given %s", err)
		return
	}

	hostname := u.Hostname()

	var robots *robotstxt.RobotsData

	if _, ok := sp.siteRobotsMap[hostname]; !ok {
		res, err := client.Get(fmt.Sprintf("https://%s/robots.txt", hostname))

		if err != nil {
			// might not have robots.txt
			log.Println("Error fetching", err.Error())

			if res.StatusCode != 200 {
				log.Printf("status code error: %d %s", res.StatusCode, res.Status)
			}
		} else {
			defer res.Body.Close()

			// has a robots.txt file
			robots, err = robotstxt.FromResponse(res)

			if err != nil {
				log.Println("Error parsing robots.txt:", err.Error())
				return
			}

			group := robots.FindGroup("*")
			sp.siteRobotsMap[hostname] = group
		}
	}

	if !shouldCrawl(sp.seed) {
		return
	}

	res, err := client.Get(sp.seed)
	if err != nil {

		log.Fatalf("fetching seed (%s) not working: %s, %s", sp.seed, err, shouldCrawl(sp.seed))
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		if res.StatusCode == 429 {
			log.Printf("Rate limited")
		}
		return
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("goquery error: %s", err)
		return
	}

	var foundUrls []string = make([]string, 0)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")

		if !shouldCrawl(href) {
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
		group, ok := sp.siteRobotsMap[hostname]
		if ok {
			allowed = group.Test(href)
		}

		if robots != nil {
			group := robots.FindGroup("*")
			allowed = group.Test(href)
		}

		if allowed && exists && (parsedHref.Scheme == "http" || parsedHref.Scheme == "https") {
			foundUrls = append(foundUrls, href)
		}
	})

	slices.Sort(foundUrls)
	foundUrls = slices.Compact(foundUrls)

	sp.linkChan <- SiteAdjacency{
		sp.seed,
		foundUrls,
	}
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

func shouldCrawl(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())

	// Block any host that starts with "login." — auth subdomains, not content
	if strings.HasPrefix(host, "login.") || strings.HasPrefix(host, "auth.") || strings.HasPrefix(host, "accounts.")  ||  strings.HasPrefix(host, "app.") {
		return false
	}

	// Extension check — only on the path
	extPattern := regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|css|js|pdf|zip)(\?|$)`)
	if extPattern.MatchString(u.Path) {
		return false
	}

	// Path-segment check — only on the path, not the host
	pathPattern := regexp.MustCompile(`(?i)/(login|cart|admin|wp-admin)(/|$)`)
	if pathPattern.MatchString(u.Path) {
		return false
	}

	return true
}
