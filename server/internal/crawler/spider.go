package crawler

import (
	"context"
	"mime"
	"net/http"

	"go.uber.org/zap"
)

type Spider struct {
	id           int
	jobs         <-chan Payload
	sendChan     chan<- *Page
	httpFailChan chan<- Payload
	log          *zap.Logger
	client       *http.Client
}

var allowedContentTypes = []string{
	"text/html",
	"text/plain",
	"application/pdf",
}

func NewSpider(id int, log *zap.Logger, client *http.Client, jobs <-chan Payload, sendChan chan<- *Page, httpFailChan chan<- Payload, dialerContext DialerContext) *Spider {
	return &Spider{
		id,
		jobs,
		sendChan,
		httpFailChan,
		log,
		client,
	}
}

func (sp *Spider) Walk(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-sp.jobs:
			if !ok {
				return
			}
			//sp.log.Debug("Job Accquired", zap.String("url", job.url))

			req, _ := http.NewRequestWithContext(ctx, "GET", job.url.String(), nil)
			mimicBrowser(req)

			res, err := sp.client.Do(req)
			if err != nil {
				//will return an error (canceled) if the ctx done channel is closed
				if ctx.Err() != nil {
					return
				}

				select {
				case <-ctx.Done():
					continue
				case sp.httpFailChan <- job:
					sp.log.Warn("Could not request site", zap.String("url", job.url.String()))
					continue
				}
			}

			success := func() bool {
				//defer before next iteration
				defer res.Body.Close()
				contentType := res.Header.Get("Content-Type")

				if !isValidContentType(contentType) {
					select {
					case <-ctx.Done():
						return false
					case sp.httpFailChan <- job:
						return false
					}
					//sp.log.Warn("Invalid content", zap.String("url", job.url.String()), zap.String("type", contentType))
				}

				var page *Page

				if !isPDF(contentType) {
					page, err = NewHTMLParser(sp.log).Parse(ctx, res, job)

					if err != nil {
						if ctx.Err() == nil {
							sp.log.Error("Goquery error", zap.Error(err), zap.String("url", job.url.String()))
						}

						return false
					}
				} else {
					page = &Page{
						URL:       job.url,
						Host:      job.host,
						InEnglish: false,
					}
					//TODO: pdf extraction
				}

				//TODO: implement overflow
				select {
				case <-ctx.Done():
					return true
				case sp.sendChan <- page:
					return true
				}

			}()

			if !success {
				return
			}

		}
	}
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

func mimicBrowser(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	req.Header.Set("Sec-Ch-Ua", `"Not-A.Brand";v="99", "Chromium";v="124"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
}
