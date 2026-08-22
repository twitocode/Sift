package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/ranker"
	"github.com/twitocode/sift/internal/services"
	"go.uber.org/zap"
	//"github.com/twitocode/sift/internal/db"
)

type Services struct {
	Search *services.SearchService
}

func NewServices(cfg *common.Config, pool *pgxpool.Pool, log *zap.Logger, ranker *ranker.Ranker) *Services {
	//queries := db.New(pool)

	return &Services{
		Search: services.NewSearchService(ranker),
	}
}
