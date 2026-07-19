package crawler

import "go.uber.org/zap"

type Spider struct {
	id       int
	jobs     <-chan SpiderPayload
	sendChan chan<- PageMetadata

	log *zap.Logger
}

func NewSpider(id int, log *zap.Logger, jobs <-chan SpiderPayload, sendChan chan<- PageMetadata) *Spider {
	return &Spider{
		id,
		jobs,
		sendChan,
		log,
	}
}

func (sp *Spider) Walk() {
	for job := range sp.jobs {
		sp.log.Debug("Job", zap.String("url", job.url))
    
		sp.sendChan <- PageMetadata{
      URL: job.url,
      Host: job.host,
      Links: []string{job.url},
		}
    
    sp.log.Debug("Finished Job", zap.String("url", job.url))
	}
}
