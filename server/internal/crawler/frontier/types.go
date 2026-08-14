package frontier

import (
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/networking"
)

type SpiderJob struct {
	url           common.URL
	hostname      common.URL
	dialerContext networking.DialerContext
}
