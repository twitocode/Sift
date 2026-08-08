package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
	//"github.com/twitocode/sift/internal/db"
)

type Services struct {
}

func NewServices(cfg *common.Config, pool *pgxpool.Pool, log *zap.Logger) *Services {
	//queries := db.New(pool)

	return &Services{}
}
