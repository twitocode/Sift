package crawler

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/db"
	"go.uber.org/zap"
)

type PageRepository struct {
	sqliteDb   *sql.DB
	queries    *db.Queries
	bufferChan chan *PageMetadata
	buffer     []*PageMetadata
  bufferSize int
	log        *zap.Logger

	mu sync.Mutex
}

func NewPageRepository(sqliteDb *sql.DB, log *zap.Logger) *PageRepository {
  bufferSize := 200
	return &PageRepository{
		queries:    db.New(sqliteDb),
		bufferChan: make(chan *PageMetadata),
		sqliteDb:   sqliteDb,
		buffer:     make([]*PageMetadata, 0, bufferSize),
		log:        log,
	}
}

func (sr *PageRepository) RunTimer(ctx context.Context) {
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
				if len(sr.buffer) >= sr.bufferSize {
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

func (sr *PageRepository) flush(ctx context.Context) {
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
func (sr *PageRepository) Add(ctx context.Context, sm PageMetadata) error {
	sr.bufferChan <- &sm

	return nil
}

func (sr *PageRepository) Contains(ctx context.Context, url URL) (bool, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	exists, err := sr.queries.FindPage(ctx, url.String())

	return exists != 0, err
}
func (sr *PageRepository) Get(ctx context.Context, url URL) (*PageMetadata, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	page, err := sr.queries.GetPageInfo(ctx, url.String())

	if err != nil {
		return nil, err
	}

	siteInfo := &PageMetadata{
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
