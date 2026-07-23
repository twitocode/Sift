package crawler

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/db"
	"go.uber.org/zap"
)

type PageStore struct {
	sqliteDb   *sql.DB
	queries    *db.Queries
	bufferChan chan *PageMetadata
	buffer     []*PageMetadata
	bufferSize int
	log        *zap.Logger

	mu sync.Mutex
}

func NewPageStore(sqliteDb *sql.DB, log *zap.Logger) *PageStore {
	bufferSize := 256
	return &PageStore{
		queries:    db.New(sqliteDb),
		bufferChan: make(chan *PageMetadata, bufferSize*2),
		sqliteDb:   sqliteDb,
		buffer:     make([]*PageMetadata, 0, bufferSize),
		log:        log,
	}
}

func (ps *PageStore) RunTimer(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(time.Second * 4)
	defer ticker.Stop()

	runFlush := func() {
		newCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		ps.flush(newCtx)
	}

	drain := func() {
		for meta := range ps.bufferChan {
			ps.buffer = append(ps.buffer, meta)
		}
	}

	ps.log.Info("Started Batcher")
	for {
		select {
		case <-ctx.Done():
			drain()
			runFlush()
			wg.Done()
			return
		case meta, ok := <-ps.bufferChan:
			if !ok {
				drain()
				runFlush()
				wg.Done()
				return
			}

			ps.buffer = append(ps.buffer, meta)
			if len(ps.buffer) >= ps.bufferSize {
				runFlush()
			}

		case <-ticker.C:
			ps.log.Debug("Flushed buffer")
			runFlush()
		}
	}
}

func (ps *PageStore) flush(ctx context.Context) {
	for _, sm := range ps.buffer {
		err := ps.queries.SetPageInfo(ctx, db.SetPageInfoParams{
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
			ps.log.Error("Sqlite write error", zap.Error(err))
		}
	}

	ps.buffer = ps.buffer[:0]
}

func (ps *PageStore) Shutdown() {
	close(ps.bufferChan)
}

// doing a direct insert for now
func (ps *PageStore) Add(ctx context.Context, sm PageMetadata) error {
	select {
	case <-ctx.Done():
		return nil
	case ps.bufferChan <- &sm:
		return nil
	}
}

func (ps *PageStore) Contains(ctx context.Context, url URL) (bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	exists, err := ps.queries.FindPage(ctx, url.String())

	return exists != 0, err
}

func (ps *PageStore) Get(ctx context.Context, url URL) (*PageMetadata, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	page, err := ps.queries.GetPageInfo(ctx, url.String())

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
