package crawler

import (
	"fmt"
	"slices"
	"sync"

	"go.uber.org/zap"
)

type Crawler struct {
	// TODO: make concurrent
	queue    []string
	maxDepth int
	graph    SiteGraph
	log      *zap.Logger

	sync.RWMutex
}

func New(logger *zap.Logger) *Crawler {
	return &Crawler{
		maxDepth: 3,
		queue:    seed,
		graph:    SiteGraph{},
		log:      logger,
	}
}

func (c *Crawler) Start() {
  c.log.Info("Starting logging")
	var linkChan chan SiteAdjacency
	linkChan = make(chan SiteAdjacency)

	for passes := 0; passes < c.maxDepth; passes++ {
		wg := sync.WaitGroup{}

		for _, seed := range c.queue {
			spider := NewSpider(seed, c.graph, linkChan)
			wg.Add(1)

			go func() {
				spider.Run()
				defer wg.Done()
			}()
		}

		go func() {
			c.receiveLinks(linkChan)
		}()
		wg.Wait()

		fmt.Printf("finished pass %d\n", passes)
	}

	close(linkChan)
	fmt.Println("Done")
}

func (c *Crawler) receiveLinks(linkChan <-chan SiteAdjacency) {
	for sa := range linkChan {
		c.Lock()

		if _, ok := c.graph[sa.site]; !ok {
			c.graph[sa.site] = sa.links
			//fmt.Printf("%s added\n", sa.site)
		}

		for _, link := range sa.links {
			if _, ok := c.graph[link]; !ok && !slices.Contains(c.queue, link) {
				c.queue = append(c.queue, link)
			}
		}
		c.Unlock()
	}
}
