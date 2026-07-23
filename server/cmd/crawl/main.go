package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/twitocode/sift/internal/app"
	"github.com/twitocode/sift/internal/crawler"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func main() {

	log := app.NewLogger(os.Getenv, zap.InfoLevel)
	app.NewConfig(os.Getenv)

	sqliteDb, err := sql.Open("sqlite", "../../db/sqlite/sift.db")

	if err != nil {
		log.Fatal("Sqlite connection error", zap.Error(err))
	}
	log.Info("Connected to Sqlite")

	pageRepo := crawler.NewPageRepository(sqliteDb, log)

	crawler.NewEngine(log, pageRepo).Start()
	fmt.Scanln()
}
