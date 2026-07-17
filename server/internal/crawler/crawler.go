package crawler

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/temoto/robotstxt"
	"go.uber.org/zap"
)

type RobotsChanInfo struct {
	name   string
	robots robotstxt.RobotsData
}

type SiteMetadata struct {
	URL string
}
type Crawler struct {
	// TODO: make concurrent
	queue    ProcessingQueue
	maxDepth int
	log      *zap.Logger

	//domain as key
	robots *SafeMap[string, robotstxt.RobotsData]
	//url as key
	crawledSites *SafeMap[string, SiteMetadata]
	sync.RWMutex
}

func New(logger *zap.Logger) *Crawler {
	return &Crawler{
		maxDepth:     3,
		queue:        ProcessingQueue{},
		log:          logger,
		robots:       NewSafeMap[string, robotstxt.RobotsData](),
		crawledSites: NewSafeMap[string, SiteMetadata](),
	}
}

func (c *Crawler) Start() {
	c.log.Info("Starting logging")

	ctx, cancel := context.WithCancel(context.Background())
	processingQueue := NewProcessingQueue(512, c.log)

	robotsChan := make(chan RobotsChanInfo, 512)

	for _, url := range seed {
		processingQueue.Push(Job{
			url,
		})
	}

	go processingQueue.Run(ctx, func(job Job) {
		spider := NewSpider(ctx, c.log, job, processingQueue, c.robots, robotsChan, c.crawledSites)
		result := spider.Run()

		if result == nil {
			return
		}

		c.crawledSites.Set(result.URL, *result)
	})

	for {
		info, ok := <-robotsChan
		if !ok {
			close(robotsChan)
		}

		c.Lock()
		if _, ok := c.robots.Get(info.name); !ok {
			c.robots.Set(info.name, info.robots)
			c.log.Debug("Robots collection updated", zap.String("hostname", info.name))
		}

		c.Unlock()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh //waits for a message from the operating system (blocking)

	cancel()
	close(robotsChan)
	c.log.Info("Done")
}
