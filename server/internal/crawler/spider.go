package crawler

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type Spider struct {
	id       int
	jobs     <-chan SpiderPayload
	sendChan chan<- *PageMetadata
	log      *zap.Logger
	client   *http.Client
}

func NewSpider(id int, log *zap.Logger, jobs <-chan SpiderPayload, sendChan chan<- *PageMetadata, dialerContext DialerContext) *Spider {
	return &Spider{
		id,
		jobs,
		sendChan,
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
			sp.log.Debug("Job Accquired", zap.String("url", job.url))

			req, _ := http.NewRequest("GET", job.url, nil)
			res, err := sp.client.Do(req)
			if err != nil {
				sp.log.Error("Could not request site", zap.String("url", job.url))
				continue
			}

			foundUrls := sp.findLinks(res)

			sp.sendChan <- &PageMetadata{
				URL:   job.url,
				Host:  job.host,
				Links: foundUrls,
			}

			sp.log.Debug("Finished Job", zap.String("url", job.url))
		}

	}
}

func (sp *Spider) findLinks(res *http.Response) []string {
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		sp.log.Error("Goquery error", zap.Error(err))
		return nil
	}

	var foundUrls []string = make([]string, 0)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		if href == "" {
			return
		}

		_, err := url.Parse(href)
		if err != nil {
			return
		}
		foundUrls = append(foundUrls, href)
	})

	slices.Sort(foundUrls)
	foundUrls = slices.Compact(foundUrls)
	return foundUrls
}
