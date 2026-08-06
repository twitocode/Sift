package indexer

import (
	"context"
	"slices"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler"
	"go.uber.org/zap"
)


type Indexer struct {
	log   *zap.Logger
	store *crawler.PageStore

	index *common.SafeMap[string, []Posting]
}

func NewIndexer(log *zap.Logger, store *crawler.PageStore) *Indexer {
	return &Indexer{
		log:   log,
		store: store,
		index: common.NewSafeMap[string, []Posting](),
	}
}

func (in *Indexer) Start(ctx context.Context) {
  //TODO: make indexer run on a ticker (or cron job)
  //TODO: add a vector store - calculate embeddings alongside invert index
	pages, err := in.store.GetAll(ctx)

	start := time.Now()
	if err != nil {
		in.log.Fatal("Aborted Indexing", zap.Error(err))
	}

	for _, page := range pages {
		in.Index(ctx, page)
	}

	// i := 0
	// in.index.Range(func(k string, v []Posting) bool {
	//   if i == 100 {
	//     return false
	//   }

	//   i++
	//   j := 0
	// 	fmt.Printf("%s: [%s]\n", k, strings.Join(common.Map(v, func(e Posting) (string, bool) {
	//     if j == 6 {
	//       return "", false
	//     }

	//     j++
	// 		return e.DocID, true
	// 	}), ", "))
	// 	return true
	// })

	elapsed := time.Since(start)
	in.log.Info("Finished indexing", zap.Duration("elapsed", elapsed))
}

func (in *Indexer) Index(ctx context.Context, page *crawler.Page) {
	textTokens := Tokenize(page.Text)
	titleTokens := Tokenize(page.Title)

	titlesStartIndex := len(textTokens)
	allTokens := slices.Concat(textTokens, titleTokens)

	postingMap := make(map[string]Posting)

	for i, token := range allTokens {
		if entry, ok := postingMap[token]; !ok {
			postingMap[token] = Posting{
				Frequency:    1,
				DocID:        string(page.URL),
				MatchesTitle: i >= titlesStartIndex,
			}
			continue
		} else {
			entry.Frequency += 1
			if !entry.MatchesTitle {
				entry.MatchesTitle = i >= titlesStartIndex
			}

			postingMap[token] = entry
		}
	}

	for token, posting := range postingMap {
		postings, ok := in.index.Get(token)
		if !ok {
			in.index.Set(token, []Posting{posting})
			continue
		}

		postings = append(postings, posting)
		in.index.Set(token, postings)
	}
}
