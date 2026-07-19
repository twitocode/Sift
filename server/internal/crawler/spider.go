package crawler

import (
	"net/http"
	"time"

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

func (sp *Spider) Walk() {
	for job := range sp.jobs {
		sp.log.Debug("Job Accquired", zap.String("url", job.url))

    req, _ := http.NewRequest("GET", job.url, nil)
    _, err := sp.client.Do(req)
    if err != nil {
      panic("bro")
    }


		sp.sendChan <- &PageMetadata{
			URL:   job.url,
			Host:  job.host,
			Links: []string{job.url},
		}

		sp.log.Debug("Finished Job", zap.String("url", job.url))
	}
}

func newHttpClient(dialerContext DialerContext) *http.Client {
	transport := &http.Transport{
		DialContext: dialerContext,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}
