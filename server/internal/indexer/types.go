package indexer

type Posting struct {
  DocID int64
  Frequency uint64
  
  //ranks higher
  MatchesTitle bool
}