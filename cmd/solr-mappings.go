package main

import (
	"errors"
	"fmt"
	"net/http"
)

// functions that map virgo data into solr data

func (s *solrRequest) buildFilters(filterGroups *VirgoFilters, availableFacets map[string]solrRequestFacet) {
	if filterGroups == nil {
		return
	}

	if len(*filterGroups) == 0 {
		return
	}

	// we are guaranteed to only have one filter group due to up-front validations

	filterGroup := (*filterGroups)[0]

	s.meta.client.log("filter group: [%s]", filterGroup.PoolID)

	for _, filter := range filterGroup.Facets {
		solrFacet, ok := availableFacets[filter.FacetID]

		// this should never happen due to up-front validations, perhaps this can be removed
		if ok == false {
			warning := fmt.Sprintf("ignoring unrecognized filter: [%s]", filter.FacetID)
			s.meta.client.log(warning)
			s.meta.warnings = append(s.meta.warnings, warning)
			continue
		}

		// remove this selected filter if it depends on other filters, none of which are selected

		if len(solrFacet.config.DependentFacetXIDs) > 0 {
			numSelected := 0

			for _, facet := range solrFacet.config.DependentFacetXIDs {
				n := len(s.meta.selectionMap[facet])
				s.meta.client.log("buildFilters(): [%s] %d selected filters for %s", filter.FacetID, n, facet)
				numSelected += n
			}

			if numSelected == 0 {
				s.meta.client.log("buildFilters(): [%s] omitting filter due to lack of selected dependent filters", filter.FacetID)
				continue
			}

			s.meta.client.log("buildFilters(): [%s] including filter due to %d selected dependent filters", filter.FacetID, numSelected)
		}

		solrFilter := fmt.Sprintf(`%s:"%s"`, solrFacet.Field, filter.Value)

		s.json.Params.Fq = append(s.json.Params.Fq, solrFilter)

		// add this filter to selection map
		if s.meta.selectionMap[filter.FacetID] == nil {
			s.meta.selectionMap[filter.FacetID] = make(map[string]bool)
		}

		s.meta.selectionMap[filter.FacetID][filter.Value] = true
	}
}

func (s *searchContext) solrAvailableFacets() map[string]solrRequestFacet {
	// build customized/personalized available facets from facets definition

	availableFacets := make(map[string]solrRequestFacet)

	auth := s.virgoReq.meta.client.isAuthenticated()

	for _, facet := range s.pool.maps.availableFacets {
		f := solrRequestFacet{
			Type:   facet.Type,
			Field:  facet.Field,
			Sort:   facet.Sort,
			Offset: facet.Offset,
			Limit:  facet.Limit,
			config: &facet,
		}

		if facet.FieldAuth != "" {
			if auth == true {
				f.Field = facet.FieldAuth
				s.log("[FACET] authenticated session using facet: [%s]", f.Field)
			} else {
				s.log("[FACET] unauthenticated session using facet: [%s]", f.Field)
			}
		}

		availableFacets[facet.XID] = f
	}

	return availableFacets
}

func (s *searchContext) solrRequestWithDefaults() searchResponse {
	var solrReq solrRequest

	solrReq.meta.client = s.virgoReq.meta.client
	solrReq.meta.parserInfo = s.virgoReq.meta.parserInfo

	solrReq.meta.selectionMap = make(map[string]map[string]bool)

	// fill out requested/defaulted sort info

	sort := VirgoSort{
		SortID: "SortRelevance",
		Order:  "desc",
	}

	if s.virgoReq.Sort != nil && (s.virgoReq.Sort.SortID != "" || s.virgoReq.Sort.Order != "") {
		// sort was specified

		sortValid := false

		if s.pool.maps.sortFields[s.virgoReq.Sort.SortID] != "" {
			// sort id is valid

			if s.virgoReq.Sort.Order == "asc" || s.virgoReq.Sort.Order == "desc" {
				// sort order is valid

				sortValid = true
				sort = *s.virgoReq.Sort
			}
		}

		if sortValid == false {
			return searchResponse{status: http.StatusBadRequest, err: errors.New("Invalid sort")}
		}
	}

	solrReq.meta.sort = sort

	// fill out as much as we can for a generic request

	solrReq.json.Params.Q = s.virgoReq.meta.solrQuery
	solrReq.json.Params.Qt = s.pool.config.Solr.Params.Qt
	solrReq.json.Params.DefType = s.pool.config.Solr.Params.DefType
	solrReq.json.Params.Fq = nonemptyValues(s.pool.config.Solr.Params.Fq)
	solrReq.json.Params.Fl = nonemptyValues(s.pool.config.Solr.Params.Fl)
	solrReq.json.Params.Start = restrictValue("start", s.virgoReq.Pagination.Start, 0, 0)
	solrReq.json.Params.Rows = restrictValue("rows", s.virgoReq.Pagination.Rows, 0, 0)
	solrReq.json.Params.Sort = fmt.Sprintf("%s %s", s.pool.maps.sortFields[solrReq.meta.sort.SortID], solrReq.meta.sort.Order)

	if s.client.opts.grouped == true {
		grouping := fmt.Sprintf("{!collapse field=%s}", s.pool.config.Solr.GroupField)
		solrReq.json.Params.Fq = append(solrReq.json.Params.Fq, grouping)
	}

	// add facets/filters

	availableFacets := s.solrAvailableFacets()

	if s.virgoReq.meta.requestFacets == true && len(availableFacets) > 0 {
		solrReq.json.Facets = availableFacets
	}

	solrReq.buildFilters(s.virgoReq.Filters, availableFacets)

	s.solrReq = &solrReq

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) solrSearchRequest() searchResponse {
	var err error

	var p *solrParserInfo

	// caller might have already supplied a Solr query
	if s.virgoReq.meta.solrQuery == "" {
		if p, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
			return searchResponse{status: http.StatusInternalServerError, err: fmt.Errorf("Virgo query to Solr conversion error: %s", err.Error())}
		}

		s.virgoReq.meta.solrQuery = p.query
		s.virgoReq.meta.parserInfo = p
	}

	if resp := s.solrRequestWithDefaults(); resp.err != nil {
		return resp
	}

	return searchResponse{status: http.StatusOK}
}
