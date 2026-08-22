package common

type SearchResult struct {
	Title   string `json:"title"`
	OGTitle string `json:"og_title"`
	Desc    string `json:"desc"`
	Favicon string `json:"favicon"`
	Url     string `json:"url"`
}
