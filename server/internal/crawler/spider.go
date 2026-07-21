package crawler

import (
	"context"
	"mime"
	"net/http"
	"slices"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type SpiderPayload struct {
	url           string
	host          string
	dialerContext DialerContext
}

type Spider struct {
	id           int
	jobs         <-chan SpiderPayload
	sendChan     chan<- *PageMetadata
	httpFailChan chan<- string
	log          *zap.Logger
	client       *http.Client
}

var allowedContentTypes = []string{
	"text/html",
	"text/plain",
	"application/pdf",
}

func NewSpider(id int, log *zap.Logger, jobs <-chan SpiderPayload, sendChan chan<- *PageMetadata, httpFailChan chan<- string, dialerContext DialerContext) *Spider {
	return &Spider{
		id,
		jobs,
		sendChan,
		httpFailChan,
		log,
		newHttpClient(dialerContext),
	}
}

func (sp *Spider) Walk(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-sp.jobs:
			//sp.log.Debug("Job Accquired", zap.String("url", job.url))

			req, _ := http.NewRequest("GET", job.url, nil)
			res, err := sp.client.Do(req)
			if err != nil {
				sp.log.Warn("Could not request site", zap.String("url", job.url))
				sp.httpFailChan <- job.host
				continue
			}

			defer res.Body.Close()
			contentType := res.Header.Get("Content-Type")

			if !isValidContentType(contentType) {
				sp.log.Warn("Invalid content", zap.String("url", job.url), zap.String("type", contentType))
				return
			}

			foundUrls := make([]string, 0)
			if !isPDF(contentType) {
				foundUrls = sp.findLinks(res, job.url)
			} else {
				//TODO: pdf extraction
			}

			pageMeta := &PageMetadata{
				URL:   job.url,
				Host:  job.host,
				Links: foundUrls,
			}

			//TODO: implement overflow

			sp.sendChan <- pageMeta

		}
	}
}

func (sp *Spider) findLinks(res *http.Response, pageURL string) []string {
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		sp.log.Error("Goquery error", zap.Error(err), zap.String("url", pageURL))
		return make([]string, 0)
	}

	var foundUrls []string = make([]string, 0)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		url, err := ResolveUrl(href, pageURL)
		if err != nil {
			return
		}

		foundUrls = append(foundUrls, url)
	})

	slices.Sort(foundUrls)
	foundUrls = slices.Compact(foundUrls)
	return foundUrls
}

func isValidContentType(contentType string) bool {
	// Content-Type headers can include params, e.g. "text/html; charset=utf-8"
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	for _, valid := range allowedContentTypes {
		if mediaType == valid {
			return true
		}
	}
	return false
}

func isPDF(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	return mediaType == "application/pdf"
}
