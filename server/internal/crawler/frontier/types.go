package frontier

import (
	"context"
	"net"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/networking"
)

type SpiderJob struct {
	url           common.URL
	hostname      common.URL
	dialerContext networking.DialerContext
}

type dnsCache interface {
	FailedUntil(host string) (time.Time, bool)
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}
