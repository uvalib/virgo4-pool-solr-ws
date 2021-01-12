package main

import (
	"errors"
	"log"
	"time"

	"github.com/uvalib/virgo4-api/v4api"
)

type facetCache struct {
	searchCtx       *searchContext
	refreshInterval int
	currentFacets   *[]v4api.Facet
	facetMap        map[string]*v4api.Facet
}

func newFacetCache(pool *poolContext, interval int) *facetCache {
	f := facetCache{
		refreshInterval: interval,
		currentFacets:   nil,
		facetMap:        nil,
	}

	// create a search context

	c := clientContext{}
	c.init(pool, nil)
	//c.opts.verbose = true

	s := searchContext{}
	s.init(pool, &c)

	s.virgo.endpoint = "internal"

	s.virgo.req.Query = "keyword:{*}"
	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 0}
	s.virgo.flags.requestFacets = true
	s.virgo.flags.facetCache = true

	f.searchCtx = &s

	go f.monitorFacets()

	return &f
}

func (f *facetCache) monitorFacets() {
	for {
		f.refreshFacets()
		f.searchCtx.log("[CACHE] refresh scheduled in %d seconds", f.refreshInterval)
		time.Sleep(time.Duration(f.refreshInterval) * time.Second)
	}
}

func (f *facetCache) refreshFacets() {
	log.Printf("[CACHE] refreshing solr facets...")

	if resp := f.searchCtx.getPoolQueryResults(); resp.err != nil {
		f.searchCtx.err("[CACHE] query error: %s", resp.err.Error())
		return
	}

	f.currentFacets = &f.searchCtx.virgo.poolRes.FacetList

	f.facetMap = make(map[string]*v4api.Facet)
	for i := range *f.currentFacets {
		facet := &(*f.currentFacets)[i]
		f.facetMap[facet.ID] = facet
	}
}

func (f *facetCache) getPreSearchFilters() ([]v4api.Facet, error) {
	// create copy of memory reference in case lists updates while we are running
	currentFacetMap := f.facetMap

	if f.currentFacets == nil {
		return nil, errors.New("facets have not been cached yet")
	}

	var filters []v4api.Facet

	for _, xid := range f.searchCtx.pool.config.Global.Mappings.Configured.FilterXIDs {
		filter := currentFacetMap[xid]

		// assume any missing filters are due to them not existing in solr
		if filter == nil {
			continue
		}

		filters = append(filters, *filter)
	}

	return filters, nil
}
