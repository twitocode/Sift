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
	bufferChan chan *Page
	buffer     []*Page
	bufferSize int
	log        *zap.Logger

	mu sync.Mutex
}

func NewPageStore(sqliteDb *sql.DB, log *zap.Logger) *PageStore {
	bufferSize := 256
	return &PageStore{
		queries:    db.New(sqliteDb),
		bufferChan: make(chan *Page, bufferSize*2),
		sqliteDb:   sqliteDb,
		buffer:     make([]*Page, 0, bufferSize),
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
		for page := range ps.bufferChan {
			ps.buffer = append(ps.buffer, page)
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
		case page, ok := <-ps.bufferChan:
			if !ok {
				drain()
				runFlush()
				wg.Done()
				return
			}

			ps.buffer = append(ps.buffer, page)
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
				Valid:  true,
			},
			Description: sql.NullString{
				String: sm.Description,
				Valid:  true,
			},
			Text: sql.NullString{
				String: sm.Text,
				Valid:  true,
			},

			StatusCode: sql.NullInt64{
				Int64: int64(sm.StatusCode),
				Valid: true,
			},
			CrawledAt: sql.NullTime{
				Valid: true,
				Time:  time.Now(),
			},
			HasBeenCrawled: sql.NullInt64{
				Valid: true,
				Int64: BoolToInt64(sm.HasBeenCrawled),
			},
			ContentHash: sql.NullInt64{
				Valid: true,
				Int64: int64(sm.ContentHash),
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
func (ps *PageStore) Add(ctx context.Context, sm Page) error {
	select {
	case <-ctx.Done():
		return nil
	case ps.bufferChan <- &sm:
		return nil
	}
}

func (ps *PageStore) Contains(ctx context.Context, url URL) (bool, error) {
	exists, err := ps.queries.FindPage(ctx, url.String())
	return exists != 0, err
}

func (ps *PageStore) Get(ctx context.Context, url URL) (*Page, error) {
	page, err := ps.queries.GetPageInfo(ctx, url.String())

	if err != nil {
		return nil, err
	}

	pageInfo := &Page{
		ContentHash:    uint64(page.ContentHash.Int64),
		Title:          page.Title.String,
		Text:           page.Text.String,
		Description:    page.Description.String,
		URL:            URL(page.Url),
		CrawledAt:      page.CrawledAt.Time,
		StatusCode:     int(page.StatusCode.Int64),
		HasBeenCrawled: page.HasBeenCrawled.Int64 == 1,
	}

	host, _ := URL(page.Url).GetHost()
	pageInfo.Host = host
	return pageInfo, nil
}

func (ps *PageStore) GetAll(ctx context.Context) ([]*Page, error) {
	out := make([]*Page, 0)
	res, err := ps.queries.GetAllPages(ctx)

	if err != nil {
		return nil, err
	}

	for _, page := range res {
		pageInfo := &Page{
			ContentHash:    uint64(page.ContentHash.Int64),
			Title:          page.Title.String,
			Text:           page.Text.String,
			Description:    page.Description.String,
			URL:            URL(page.Url),
			CrawledAt:      page.CrawledAt.Time,
			StatusCode:     int(page.StatusCode.Int64),
			HasBeenCrawled: page.HasBeenCrawled.Int64 == 1,
		}

		host, _ := URL(page.Url).GetHost()
		pageInfo.Host = host

		out = append(out, pageInfo)
	}

	return out, nil
}

func BoolToInt64(b bool) int64 {
	if b {
		return 1
	}

	return 0
}
