package scraper

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
)

type ChainScraper struct {
	log       *zap.SugaredLogger
	searchers []Searcher
}

func NewChainScraper(log *zap.SugaredLogger, backends ...Searcher) *ChainScraper {
	var active []Searcher
	for _, b := range backends {
		if b.Enabled() {
			log.Infof("Scraper backend enabled: %s", b.Name())
			active = append(active, b)
		} else {
			log.Infof("Scraper backend disabled: %s", b.Name())
		}
	}
	return &ChainScraper{log: log, searchers: active}
}

func (c *ChainScraper) Search(opts SearchOptions) ([]SearchResult, error) {
	if len(c.searchers) == 0 {
		return nil, fmt.Errorf("no scraper backends enabled")
	}

	var errs []error
	for _, s := range c.searchers {
		results, err := s.Search(opts)
		if err == nil {
			c.log.Infof("Scrape succeeded via %s", s.Name())
			return results, nil
		}
		c.log.Warnf("Scrape failed via %s, trying next... error: %v", s.Name(), err)
		errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
	}

	return nil, errors.Join(errs...)
}
