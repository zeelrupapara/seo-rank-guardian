package scraper

type SearchOptions struct {
	Query       string
	Region      string // gl= param (e.g. "us")
	Language    string // hl= param (e.g. "en")
	ResultLimit int
}

type Searcher interface {
	Name() string
	Search(opts SearchOptions) ([]SearchResult, error)
	Enabled() bool
}
