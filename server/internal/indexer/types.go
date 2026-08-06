package indexer

type Posting struct {
  DocID string
  Frequency uint8
  
  //ranks higher
  MatchesTitle bool
}