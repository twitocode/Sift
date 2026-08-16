package frontier

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/dedup"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type Parser interface {
	Parse() *common.Page
}

type HTMLParser struct {
	log     *zap.Logger
	metrics *metrics.CrawlMetrics

	maxHTMLSize     int
	maxLinksPerPage int
}

type ParserOutput struct {
	title          string
	description    string
	text           string
	inEnglish      bool
	hasBeenCrawled bool
	foundCanonical string
	links          []common.URL
}

func NewHTMLParser(log *zap.Logger, metrics *metrics.CrawlMetrics) *HTMLParser {
	return &HTMLParser{
		log:             log,
		metrics:         metrics,
		maxHTMLSize:     10 * 1024 * 1024, // 1 MB of data,
		maxLinksPerPage: 100,
	}
}

func (p *HTMLParser) Parse(ctx context.Context, res *http.Response, job SpiderJob) (*common.Page, error) {
	if res.ContentLength > int64(p.maxHTMLSize) {
		p.log.Debug("Page Too Large", zap.String("url", job.url.String()), zap.Int64("content_length", res.ContentLength))
		return nil, fmt.Errorf("HTML Content-Length %d exceeds %d Bytes", res.ContentLength, p.maxHTMLSize)
	}

	limitReader := io.LimitReader(res.Body, int64(p.maxHTMLSize)+1)
	htmlBytes, err := io.ReadAll(limitReader)

	if err != nil {
		if ctx.Err() == nil {
			p.metrics.ParsingFailures.Add(1)
			p.log.Error("HTML read error", zap.Error(err), zap.String("url", job.url.String()))
		}
		return nil, err
	}

	if len(htmlBytes) > p.maxHTMLSize {
		p.metrics.BytesDownloaded.Add(int64(p.maxHTMLSize))
		p.log.Debug("HTML downloaded exceeds limit", zap.String("url", job.url.String()))
		return nil, fmt.Errorf("HTML requested exceeds %d Bytes", p.maxHTMLSize)
	}

	p.metrics.BytesDownloaded.Add(int64(len(htmlBytes)))

	var output ParserOutput
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		//i still want it to grab the links just not the page data
		output = p.getMeta(htmlBytes, job.url)
	} else {
		output = ParserOutput{inEnglish: false}
	}

	//TODO: add "github.com/abadojack/whatlanggo" for proper language checking

	if output.description == output.title {
		output.description = ""
	}

	page := &common.Page{
		FinalURL:     common.URL(res.Request.URL.String()),
		RequestedURL: job.url,
		Host:         job.hostname,
		Title:        output.title,
		Description:  output.description,
		CrawledAt:    time.Now(),
		InEnglish:    output.inEnglish,
		StatusCode:   res.StatusCode,
		Text:         output.text,
		// Links:          p.findLinks(doc, job.url),
		Links:          output.links,
		HasBeenCrawled: output.hasBeenCrawled,
		ContentHash:    dedup.CreateSimhashFingerprint(output.text),
		DuplicateOf:    -1,
		FoundCanonical: common.URL(output.foundCanonical).NormalizeString(),
	}

	p.metrics.PagesParsed.Add(1)
	return page, nil
}

func (p *HTMLParser) getMeta(body []byte, url common.URL) ParserOutput {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	var buffer bytes.Buffer
	var foundUrls []common.URL = make([]common.URL, 0)
	seenURLs := make(map[common.URL]struct{})

	var out ParserOutput = ParserOutput{
		inEnglish:      true,
		hasBeenCrawled: false,
		foundCanonical: "",
	}

	skipTextContent := false
	readingTitle := false
	insideBody := false
	reachedMaxLinks := false

	tagsToIgnore := []string{"script", "style", "header", "footer", "nav", "svg", "aside", "noscript", "iframe", "canvas", "embed", "form", "input", "select", "option", "label"}

	//TODO: add advertisement detection
Loop:
	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			{
				if tokenizer.Err() == io.EOF {
					out.text = buffer.String()
					out.links = foundUrls
					out.hasBeenCrawled = true
					break Loop
				}
				p.log.Error("HTML tokenizing error", zap.String("url", url.String()))
				//TODO: proper error handling
				break Loop
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			{
				token := tokenizer.Token()

				if token.Data == "html" {
					lang := getAttr(token, "lang")
					if !strings.HasPrefix(lang, "en") {
						out.inEnglish = false
					}
				}

				if slices.Contains(tagsToIgnore, token.Data) {
					skipTextContent = true
				}

				if token.Data == "body" {
					insideBody = true
				}

				if token.Data == "title" {
					readingTitle = true
				}

				if token.Data == "meta" {
					name := getAttr(token, "name")
					content := getAttr(token, "content")
					property := getAttr(token, "property")

					if (name == "description" || property == "og:description") && content != "" {
						out.description = content
					}

				}

				if token.Data == "link" {
					rel := getAttr(token, "rel")
					href := getAttr(token, "href")

					if (rel == "canonical") && href != "" {
						out.foundCanonical = href
					}
				}

				if token.Data == "a" {
					if len(foundUrls) >= p.maxLinksPerPage {
						if !reachedMaxLinks {
							p.log.Debug("Reached Max Links", zap.String("url", url.String()))
							reachedMaxLinks = true
						}
						continue
					}

					href := getAttr(token, "href")

					if href != "" {
						resolved, err := common.URL(href).ResolveUrl(url)
						if err != nil {
							continue
						}

						if _, ok := seenURLs[resolved]; ok {
							continue
						}

						foundUrls = append(foundUrls, resolved)
						seenURLs[resolved] = struct{}{}
					}
				}
			}
		case html.EndTagToken:
			{
				token := tokenizer.Token()

				if slices.Contains(tagsToIgnore, token.Data) {
					skipTextContent = false
				}

				if token.Data == "title" {
					readingTitle = false
				}

				if token.Data == "body" {
					insideBody = false
				}
			}

		case html.TextToken:
			{
				if readingTitle {
					out.title += string(tokenizer.Text()) + " "
				} else if insideBody {
					if !skipTextContent {
						chars := tokenizer.Text()
						//removes tab characters
						chars = bytes.ReplaceAll(chars, []byte{'\t', '\n'}, nil)

						normalized := normalizeExtractedText(string(chars))
						if normalized != "" {
							normalized += " "
							buffer.Write([]byte(normalized))
						}
					}
				}
			}
		}
	}

	if out.title != "" && readingTitle {
		out = ParserOutput{
			inEnglish:      true,
			hasBeenCrawled: false,
			foundCanonical: "",
			links:          foundUrls,
		}

	}
	return out
}

func getAttr(t html.Token, targetKey string) string {
	for _, attr := range t.Attr {
		if attr.Key == targetKey {
			return attr.Val
		}
	}
	return ""
}

func normalizeExtractedText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
