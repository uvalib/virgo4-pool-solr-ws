package main

import (
	"errors"
	"log"
	"sort"
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

	s := searchContext{}
	s.init(pool, &c)

	s.virgo.endpoint = "internal"

	s.virgo.req.Query = "keyword:{*}"
	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 0}
	s.virgo.flags.requestFacets = true
	s.virgo.flags.allSearchFilters = true

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

func (f *facetCache) overlayFilters(facets *[]v4api.Facet) ([]v4api.Facet, error) {
	// create copy of memory reference in case lists updates while we are running
	currentFacetMap := f.facetMap

	if f.currentFacets == nil {
		return nil, errors.New("facets have not been cached yet")
	}

	// TODO: for each passed facet:
	// * overlay its bucket values/counts onto the cached list of all values (zeroed out)
	// * sort these values according to the facet's Sort value

	var filters []v4api.Facet

	for i := range *facets {
		facet := &(*facets)[i]

		filter := currentFacetMap[facet.ID]

		// if there is no cached filter, just use what we have (it will be sorted correctly)
		if filter == nil {
			filters = append(filters, *facet)
			continue
		}

		// otherwise, explode all the values into a map, keeping track of counts, and sort the results

		bucketMap := make(map[string]v4api.FacetBucket)

		// assign cached facet values a zero count
		for i := range filter.Buckets {
			bucket := &filter.Buckets[i]
			bucketMap[bucket.Value] = v4api.FacetBucket{Value: bucket.Value, Count: 0, Selected: false}
		}

		// assign passed facet values their counts/selection status
		for i := range facet.Buckets {
			bucket := &facet.Buckets[i]
			bucketMap[bucket.Value] = v4api.FacetBucket{Value: bucket.Value, Count: bucket.Count, Selected: bucket.Selected}
		}

		// convert bucket map to bucket list
		var buckets []v4api.FacetBucket
		for _, value := range bucketMap {
			buckets = append(buckets, value)
		}

		// sort bucket list
		switch facet.Sort {
		case "alpha":
			sort.Slice(buckets, func(i, j int) bool {
				// bucket values are unique so this is the only test we need
				return buckets[i].Value < buckets[j].Value
			})

		default:
			// sort by count
			sort.Slice(buckets, func(i, j int) bool {
				if buckets[i].Count > buckets[j].Count {
					return true
				}

				if buckets[i].Count < buckets[j].Count {
					return false
				}

				// items with the same count get sorted alphabetically for consistency
				return buckets[i].Value < buckets[j].Value
			})
		}

		// assign new buckets to the facet
		facet.Buckets = buckets
		filters = append(filters, *facet)
	}

	return filters, nil
}
