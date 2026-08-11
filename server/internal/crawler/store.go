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
	bufferSize := 1024
	return &PageStore{
		queries:    db.New(sqliteDb),
		bufferChan: make(chan *Page, bufferSize*2),
		sqliteDb:   sqliteDb,
		bufferSize: bufferSize,
		buffer:     make([]*Page, 0, bufferSize),
		log:        log,
	}
}

func (ps *PageStore) RunTimer(ctx context.Context, wg *sync.WaitGroup, metrics *CrawlMetrics) {
	ticker := time.NewTicker(time.Second * 4)
	defer ticker.Stop()

	runFlush := func() {
		newCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		ps.flush(newCtx, metrics)
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

func (ps *PageStore) flush(ctx context.Context, metrics *CrawlMetrics) {
	tx, err := ps.sqliteDb.Begin()
	if err != nil {
		ps.log.Error("Could not create page transaction", zap.Error(err))
		return
	}
	defer tx.Rollback()

	added := int64(0)
	qtx := ps.queries.WithTx(tx)

	for _, sm := range ps.buffer {
		err := qtx.SetPageInfo(ctx, db.SetPageInfoParams{
			FinalUrl:   sm.FinalURL.String(),
			RequestUrl: sm.RequestedURL.String(),
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
			FoundCanonical: sql.NullString{
				Valid:  sm.FoundCanonical != "",
				String: sm.FoundCanonical.String(),
			},
		})

		if err != nil {
			ps.log.Error(
				"Sqlite write error",
				zap.Error(err),
				zap.String("request_url", sm.RequestedURL.String()),
				zap.String("final_url", sm.FinalURL.String()),
			)
			return
		}

		added++
	}

	err = tx.Commit()
	if err != nil {
		ps.log.Error("Sqlite transaction write error", zap.Error(err))
		return
	}
	metrics.PagesStored.Add(added)
	ps.buffer = ps.buffer[:0]
	ps.log.Info("Flushed page buffer", zap.Int64("count", added))
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
	exists, err := ps.queries.FindPageByURL(ctx, url.String())
	return exists != 0, err
}

func (ps *PageStore) Get(ctx context.Context, url URL) (*Page, error) {
	page, err := ps.queries.GetPageInfoByURL(ctx, url.String())

	if err != nil {
		return nil, err
	}

	pageInfo := &Page{
		ID:                page.ID,
		ContentHash:       uint64(page.ContentHash.Int64),
		Title:             page.Title.String,
		Text:              page.Text.String,
		Description:       page.Description.String,
		FinalURL:          URL(page.FinalUrl),
		CrawledAt:         page.CrawledAt.Time,
		StatusCode:        int(page.StatusCode.Int64),
		HasBeenCrawled:    page.HasBeenCrawled.Int64 == 1,
		FoundCanonical:    URL(page.FoundCanonical.String),
		ResolvedCanonical: page.ResolvedCanonical.Int64 == 1,
	}

	host, _ := URL(page.FinalUrl).GetHost()
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
			ID:                page.ID,
			ContentHash:       uint64(page.ContentHash.Int64),
			Title:             page.Title.String,
			Text:              page.Text.String,
			Description:       page.Description.String,
			FinalURL:          URL(page.FinalUrl),
			CrawledAt:         page.CrawledAt.Time,
			StatusCode:        int(page.StatusCode.Int64),
			HasBeenCrawled:    page.HasBeenCrawled.Int64 == 1,
			FoundCanonical:    URL(page.FoundCanonical.String),
			ResolvedCanonical: page.ResolvedCanonical.Int64 == 1,
		}

		host, _ := URL(page.FinalUrl).GetHost()
		pageInfo.Host = host

		out = append(out, pageInfo)
	}

	return out, nil
}

func (ps *PageStore) BatchAssignCanonical(ctx context.Context, canonicalId int64, duplicates []int64) {
	err := ps.queries.BatchAssignCanonical(ctx, db.BatchAssignCanonicalParams{
		DuplicateOf: sql.NullInt64{
			Valid: true,
			Int64: canonicalId,
		},

		Ids: duplicates,
	})

	if err != nil {
		ps.log.Error("Sqlite canonical write error", zap.Error(err))
		return
	}
}

func (ps *PageStore) BeforeCrawl(ctx context.Context) {
	ps.log.Info("Attempting to delete previous set")
	err := ps.queries.DeleteAllPages(ctx)
	if err != nil {
		ps.log.Fatal("Sqlite delete all error", zap.Error(err))
	}
	ps.log.Info("Successfully cleaned up frontier")
}

func (ps *PageStore) FindPossibleDuplicatePages(ctx context.Context) ([]*Page, error) {
	out := make([]*Page, 0)
	res, err := ps.queries.FindPossibleDuplicatePages(ctx)
	if err != nil {
		ps.log.Fatal("Sqlite query error", zap.Error(err))
		return nil, err
	}

	for _, page := range res {
		pageInfo := &Page{
			ID:                page.ID,
			ContentHash:       uint64(page.ContentHash.Int64),
			Title:             page.Title.String,
			Text:              page.Text.String,
			Description:       page.Description.String,
			FinalURL:          URL(page.FinalUrl),
			CrawledAt:         page.CrawledAt.Time,
			StatusCode:        int(page.StatusCode.Int64),
			HasBeenCrawled:    page.HasBeenCrawled.Int64 == 1,
			FoundCanonical:    URL(page.FoundCanonical.String),
			ResolvedCanonical: page.ResolvedCanonical.Int64 == 1,
		}

		host, _ := URL(page.FinalUrl).GetHost()
		pageInfo.Host = host

		out = append(out, pageInfo)
	}

	return out, nil
}

func (ps *PageStore) FindCanonicDuplicatesPages(ctx context.Context) ([]*Page, error) {
	out := make([]*Page, 0)
	res, err := ps.queries.FindCanonicDuplicatesPages(ctx)
	if err != nil {
		ps.log.Fatal("Sqlite query error", zap.Error(err))
		return nil, err
	}

	for _, page := range res {
		pageInfo := &Page{
			ID:                page.ID,
			ContentHash:       uint64(page.ContentHash.Int64),
			Title:             page.Title.String,
			Text:              page.Text.String,
			Description:       page.Description.String,
			FinalURL:          URL(page.FinalUrl),
			CrawledAt:         page.CrawledAt.Time,
			StatusCode:        int(page.StatusCode.Int64),
			HasBeenCrawled:    page.HasBeenCrawled.Int64 == 1,
			FoundCanonical:    URL(page.FoundCanonical.String),
			ResolvedCanonical: page.ResolvedCanonical.Int64 == 1,
		}

		host, _ := URL(page.FinalUrl).GetHost()
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
