package crawler

type SiteGraph map[string][]string

type SiteAdjacency struct {
	site  string
	links []string
}

// type SiteGraphNode struct {

// }
