package crawler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type Parser interface {
	Parse() *Page
}

type HTMLParser struct {
	log *zap.Logger
}

type ParserOutput struct {
	title          string
	description    string
	text           string
	inEnglish      bool
	hasBeenCrawled bool
}

func NewHTMLParser(log *zap.Logger) *HTMLParser {
	return &HTMLParser{log: log}
}

func (p *HTMLParser) Parse(ctx context.Context, res *http.Response, job Payload) (*Page, error) {
	htmlBytes, _ := io.ReadAll(res.Body)
	parsedHTML, err := html.Parse(bytes.NewReader(htmlBytes))

	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("HTML parsing error", zap.Error(err), zap.String("url", job.url.String()))
		}
		return nil, err
	}

	doc := goquery.NewDocumentFromNode(parsedHTML)

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
	page := &Page{
		URL:            job.url,
		Host:           job.host,
		Title:          output.title,
		Description:    output.description,
		CrawledAt:      time.Now(),
		InEnglish:      output.inEnglish,
		StatusCode:     res.StatusCode,
		Text:           output.text,
		Links:          p.findLinks(doc, job.url),
		HasBeenCrawled: output.hasBeenCrawled,
	}

	return page, nil
}

func (p *HTMLParser) findLinks(doc *goquery.Document, pageURL URL) []URL {
	var foundUrls []URL = make([]URL, 0)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		resolved, err := URL(href).ResolveUrl(pageURL)
		if err != nil {
			return
		}

		foundUrls = append(foundUrls, resolved)
	})

	slices.Sort(foundUrls)
	foundUrls = slices.Compact(foundUrls)
	return foundUrls
}

func (p *HTMLParser) getMeta(body []byte, url URL) ParserOutput {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	var buffer bytes.Buffer
	var out ParserOutput = ParserOutput{
		inEnglish:      true,
		hasBeenCrawled: false,
	}

	skipTextContent := false
	readingTitle := false
	insideBody := false

Loop:
	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			{
				if tokenizer.Err() == io.EOF {
					out.text = buffer.String()
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

				if token.Data == "script" || token.Data == "style" {
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
			}
		case html.EndTagToken:
			{
				token := tokenizer.Token()

				if token.Data == "script" || token.Data == "style" {
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
						buffer.Write(tokenizer.Text())
						buffer.WriteByte(' ')
					}
				}
			}
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
