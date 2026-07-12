package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
  	//"github.com/twitocode/sift/internal/db"

)

type Services struct {
}

func NewServices(cfg *Config, pool *pgxpool.Pool, log *zap.Logger) *Services {
	//queries := db.New(pool)

	return &Services{}
}
