package crawler

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/db"
	"go.uber.org/zap"
)

type SiteMetadata struct {
	URL         URL
	Title       string
	Description string
	Text        string
	Links       []URL
	StatusCode  int // some pages return 429 stuff like that so i can filter out later if needed
	CrawledAt   time.Time
	ContentHash string //TODO: duplication detection, hash text form page (different urls same text)
}

var bufferSize = 50

type SiteRepository struct {
	sqliteDb   *sql.DB
	queries    *db.Queries
	bufferChan chan *SiteMetadata
	buffer     []*SiteMetadata
	log        *zap.Logger

	mu sync.Mutex
}

func NewSiteRepository(sqliteDb *sql.DB, log *zap.Logger) *SiteRepository {

	return &SiteRepository{
		queries:    db.New(sqliteDb),
		bufferChan: make(chan *SiteMetadata),
		sqliteDb:   sqliteDb,
		buffer:     make([]*SiteMetadata, 0, 200),
		log:        log,
	}
}

func (sr *SiteRepository) RunTimer(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 4)
	defer ticker.Stop()

	sr.log.Info("Started Batcher")
	go func() {
		for {
			select {
			case meta, ok := <-sr.bufferChan:
				if !ok {
					sr.flush(ctx)
					return
				}

				sr.buffer = append(sr.buffer, meta)
				if len(sr.buffer) >= 200 {
					sr.flush(ctx)
				}

			case <-ticker.C:
        sr.log.Debug("Flushed buffer")
				sr.flush(ctx)
			}
		}
	}()

	<-ctx.Done()
}

func (sr *SiteRepository) flush(ctx context.Context) {
	for _, sm := range sr.buffer {
		err := sr.queries.SetPageInfo(ctx, db.SetPageInfoParams{
			Url: sm.URL.String(),
			Title: sql.NullString{
				String: sm.Title,
			},
			Text: sql.NullString{
				String: sm.Text,
			},
			StatusCode: sql.NullInt64{
				Int64: int64(sm.StatusCode),
				Valid: true,
			},
			CrawledAt: sql.NullTime{
				Valid: true,
				Time:  time.Now(),
			},
		})

		if err != nil {
			sr.log.Error("Sqlite write error", zap.Error(err))
		}
	}

	sr.buffer = sr.buffer[:0]
}

// doing a direct insert for now
func (sr *SiteRepository) Add(ctx context.Context, sm SiteMetadata) error {
	sr.bufferChan <- &sm

	return nil
}

func (sr *SiteRepository) Contains(ctx context.Context, url URL) (bool, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	exists, err := sr.queries.FindPage(ctx, url.String())

	return exists != 0, err
}
func (sr *SiteRepository) Get(ctx context.Context, url URL) (*SiteMetadata, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	page, err := sr.queries.GetPageInfo(ctx, url.String())

	if err != nil {
		return nil, err
	}

	siteInfo := &SiteMetadata{
		ContentHash: page.ContentHash.String,
		Title:       page.Title.String,
		Text:        page.Text.String,
		Description: page.Description.String,
		URL:         URL(page.Url),
		CrawledAt:   page.CrawledAt.Time,
		StatusCode:  int(page.StatusCode.Int64),
	}

	return siteInfo, nil
}
