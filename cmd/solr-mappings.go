package main

import (
	"fmt"
	"net/http"

	"github.com/uvalib/virgo4-api/v4api"
)

// functions that map virgo data into solr data

func (s *solrRequest) buildFilters(filterGroups []v4api.Filter, availableFacets map[string]solrRequestFacet, availability poolConfigAvailability) {
	if len(filterGroups) == 0 {
		return
	}

	// we are guaranteed to only have one filter group due to up-front validations

	filterGroup := filterGroups[0]

	for _, filter := range filterGroup.Facets {
		solrFacet := availableFacets[filter.FacetID]

		// remove this selected filter if it depends on other filters, none of which are selected

		if len(solrFacet.config.DependentFacetXIDs) > 0 {
			numSelected := 0

			for _, facet := range solrFacet.config.DependentFacetXIDs {
				n := len(s.meta.selectionMap[facet])
				numSelected += n
			}

			if numSelected == 0 {
				s.meta.client.log("[FILTER] omitting filter [%s] due to lack of selected dependent filters", filter.FacetID)
				continue
			}

			s.meta.client.log("[FILTER] including filter [%s] due to %d selected dependent filters", filter.FacetID, numSelected)
		}

		var solrFilter string
		var filterValue string

		switch solrFacet.config.Type {
		case "boolean":
			filterValue = solrFacet.config.Solr.Value

			if solrFacet.config.Format == "circulating" {
				availabilityFacet := availability.Anon.Facet
				if s.meta.client.isAuthenticated() == true {
					availabilityFacet = availability.Auth.Facet
				}

				solrFilter = fmt.Sprintf(`(%s:"%s") OR (%s:"Online")`, solrFacet.Field, filterValue, availabilityFacet)
			} else {
				solrFilter = fmt.Sprintf(`%s:"%s"`, solrFacet.Field, filterValue)
			}

		default:
			filterValue = filter.Value
			solrFilter = fmt.Sprintf(`%s:"%s"`, solrFacet.Field, filterValue)
		}

		s.json.Params.Fq = append(s.json.Params.Fq, solrFilter)

		// add this filter to selection map
		if s.meta.selectionMap[filter.FacetID] == nil {
			s.meta.selectionMap[filter.FacetID] = make(map[string]string)
		}

		s.meta.selectionMap[filter.FacetID][filterValue] = solrFilter
	}

	for filterID := range s.meta.selectionMap {
		for _, solrFilter := range s.meta.selectionMap[filterID] {
			s.meta.client.log("[FILTER] applying filter: %-20s : %s", filterID, solrFilter)
		}
	}
}

func (s *searchContext) solrAvailableFacets() map[string]solrRequestFacet {
	// build customized/personalized available facets from facets definition

	availableFacets := make(map[string]solrRequestFacet)

	auth := s.client.isAuthenticated()

	for i := range s.pool.maps.availableFacets {
		facet := s.pool.maps.availableFacets[i]

		f := solrRequestFacet{
			Type:   facet.Solr.Type,
			Field:  facet.Solr.Field,
			Sort:   facet.Solr.Sort,
			Offset: facet.Solr.Offset,
			Limit:  facet.Solr.Limit,
			config: &facet,
		}

		if facet.Solr.FieldAuth != "" && auth == true {
			f.Field = facet.Solr.FieldAuth
		}

		availableFacets[facet.XID] = f
	}

	return availableFacets
}

func (s *searchContext) solrRequestWithDefaults() searchResponse {
	var solrReq solrRequest

	solrReq.meta.client = s.client
	solrReq.meta.parserInfo = s.virgo.parserInfo

	solrReq.meta.selectionMap = make(map[string]map[string]string)

	solrReq.meta.sort = s.virgo.req.Sort

	// fill out as much as we can for a generic request

	solrReq.json.Params.Q = s.virgo.solrQuery
	solrReq.json.Params.Qt = s.pool.config.Local.Solr.Params.Qt
	solrReq.json.Params.DefType = s.pool.config.Local.Solr.Params.DefType
	solrReq.json.Params.Fq = nonemptyValues(s.pool.config.Local.Solr.Params.Fq)
	solrReq.json.Params.Fl = nonemptyValues(s.pool.config.Local.Solr.Params.Fl)
	solrReq.json.Params.Start = restrictValue("start", s.virgo.req.Pagination.Start, 0, 0)
	solrReq.json.Params.Rows = restrictValue("rows", s.virgo.req.Pagination.Rows, 0, 0)
	solrReq.json.Params.Sort = fmt.Sprintf("%s %s", s.pool.maps.sortFields[solrReq.meta.sort.SortID].Field, solrReq.meta.sort.Order)

	if s.client.opts.grouped == true {
		grouping := fmt.Sprintf("{!collapse field=%s}", s.pool.config.Local.Solr.GroupField)
		solrReq.json.Params.Fq = append(solrReq.json.Params.Fq, grouping)
	}

	// add facets/filters

	availableFacets := s.solrAvailableFacets()

	if s.virgo.requestFacets == true && len(availableFacets) > 0 {
		solrReq.json.Facets = availableFacets
	}

	solrReq.buildFilters(s.virgo.req.Filters, availableFacets, s.pool.config.Global.Availability)

	s.solr.req = &solrReq

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) solrSearchRequest() searchResponse {
	var err error

	var p *solrParserInfo

	// caller might have already supplied a Solr query
	if s.virgo.solrQuery == "" {
		if p, err = virgoQueryConvertToSolr(s.virgo.req.Query); err != nil {
			return searchResponse{status: http.StatusInternalServerError, err: fmt.Errorf("failed to convert Virgo query to Solr query: %s", err.Error())}
		}

		s.virgo.solrQuery = p.query
		s.virgo.parserInfo = p
	}

	if resp := s.solrRequestWithDefaults(); resp.err != nil {
		return resp
	}

	return searchResponse{status: http.StatusOK}
}
