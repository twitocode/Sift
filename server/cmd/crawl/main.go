package main

import (
	"os"

	"github.com/twitocode/sift/internal/app"
	"github.com/twitocode/sift/internal/crawler"
)

func main() {
  logger := app.NewLogger(os.Getenv)

  crawler := crawler.New(logger)
  crawler.Start()
}
