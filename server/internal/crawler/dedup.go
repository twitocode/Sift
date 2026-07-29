package crawler

import "context"

type Deduplicator struct {
  store *PageStore
  index *SimHashIndex
}

func NewDeduplicator(store *PageStore) *Deduplicator {
  return &Deduplicator{
    store: store,
    index: NewSimHashIndex(3, false),
  }
}

func (d *Deduplicator) Start(ctx context.Context) {
  
}
