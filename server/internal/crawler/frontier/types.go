package frontier

import (
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/networking"
)

type SpiderPayload struct {
	url           common.URL
	host          common.URL
	dialerContext networking.DialerContext
}
