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
	bufferSize := 256
	return &PageRepository{
		queries:    db.New(sqliteDb),
		bufferChan: make(chan *PageMetadata, bufferSize*2),
		sqliteDb:   sqliteDb,
		buffer:     make([]*PageMetadata, 0, bufferSize),
		log:        log,
	}
}

func (sr *PageRepository) RunTimer(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(time.Second * 4)
	defer ticker.Stop()

	runFlush := func() {
		newCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		sr.flush(newCtx)
	}

	drain := func() {
		for meta := range sr.bufferChan {
			sr.buffer = append(sr.buffer, meta)
		}
	}

	sr.log.Info("Started Batcher")
	for {
		select {
		case <-ctx.Done():
			drain()
			runFlush()
			wg.Done()
			return
		case meta, ok := <-sr.bufferChan:
			if !ok {
				drain()
				runFlush()
				wg.Done()
				return
			}

			sr.buffer = append(sr.buffer, meta)
			if len(sr.buffer) >= sr.bufferSize {
				runFlush()
			}

		case <-ticker.C:
			sr.log.Debug("Flushed buffer")
			runFlush()
		}
	}
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

func (sr *PageRepository) Shutdown() {
	close(sr.bufferChan)
}

// doing a direct insert for now
func (sr *PageRepository) Add(ctx context.Context, sm PageMetadata) error {
	select {
	case <-ctx.Done():
		return nil
	case sr.bufferChan <- &sm:
		return nil
	}
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
