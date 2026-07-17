package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/twitocode/sift/internal/app"
	"github.com/twitocode/sift/internal/crawler"
)

func main() {
	logger := app.NewLogger(os.Getenv)

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.Dial("udp", "8.8.8.8:53")
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := r.LookupHost(ctx, "www.google.com")
	fmt.Println(ips, err)

	crawler := crawler.New(logger)
	crawler.Start()
}
