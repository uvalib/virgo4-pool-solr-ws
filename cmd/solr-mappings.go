package main

import (
	"fmt"
	"net/http"

	"github.com/uvalib/virgo4-api/v4api"
)

// functions that map virgo data into solr data

func (s *solrRequest) buildFilters(ctx *searchContext, filterGroups []v4api.Filter, internalFacets map[string]*solrRequestFacet, availability poolConfigAvailability) {
	if len(filterGroups) == 0 {
		return
	}

	// we are guaranteed to only have one filter group due to up-front validations

	filterGroup := filterGroups[0]

	for _, filter := range filterGroup.Facets {
		solrFacet := internalFacets[filter.FacetID]
		if solrFacet == nil {
			continue
		}

		// omit this selected filter if it depends on other filters, none of which are selected

		if len(solrFacet.config.DependentFilterXIDs) > 0 {
			numSelected := 0

			for _, facet := range solrFacet.config.DependentFilterXIDs {
				n := len(s.meta.selectionMap[facet])
				numSelected += n
			}

			if numSelected == 0 {
				s.meta.client.log("FILTER: omitting filter [%s] due to lack of selected dependent filters", filter.FacetID)
				continue
			}

			s.meta.client.log("FILTER: including filter [%s] due to %d selected dependent filters", filter.FacetID, numSelected)
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

		case "component":
			filterValue = filter.Value
			q := solrFacet.config.queryMap[filterValue]

			if q == nil {
				s.meta.client.log("FILTER: unable to map component value to a component query: [%s]", filterValue)
				continue
			}

			solrFilter = q.Query

		default:
			var err error

			filterValue, err = ctx.getInternalSolrValue(solrFacet.config.Solr.Field, filter.Value)
			if err != nil {
				ctx.warn(err.Error())
				continue
			}

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
			s.meta.client.log("FILTER: applying filter: %-20s : %s", filterID, solrFilter)
		}
	}
}

func (s *searchContext) solrInternalRequestFacets() (map[string]*solrRequestFacet, map[string]*solrRequestFacet) {
	// build customized/personalized available facets from facets definition

	internalFacets := make(map[string]*solrRequestFacet)
	requestFacets := make(map[string]*solrRequestFacet)

	auth := s.client.isAuthenticated()

	// should we request facets or pre-search filters?

	var sourceFacets map[string]*poolConfigFilter
	if s.virgo.flags.preSearchFilters == true {
		sourceFacets = s.pool.maps.preSearchFilters
	} else {
		sourceFacets = s.resourceTypeCtx.filterMap
	}

	for i := range sourceFacets {
		facet := sourceFacets[i]

		f := solrRequestFacet{
			Type:   facet.Solr.Type,
			Field:  facet.Solr.Field,
			Sort:   facet.Solr.Sort,
			Offset: facet.Solr.Offset,
			Limit:  facet.Solr.Limit,
			Facet:  solrRequestSubFacet{GroupCount: fmt.Sprintf("unique(%s)", s.pool.config.Local.Solr.GroupField)},
			config: facet,
		}

		if facet.Solr.FieldAuth != "" && auth == true {
			f.Field = facet.Solr.FieldAuth
		}

		internalFacets[facet.XID] = &f

		switch facet.Type {
		case "component":
			for _, q := range facet.ComponentQueries {
				qf := f
				qf.Query = q.Query
				requestFacets[q.XID] = &qf
			}

		default:
			requestFacets[facet.XID] = &f
		}
	}

	return internalFacets, requestFacets
}

func (s *searchContext) solrRequestWithDefaults() searchResponse {
	s.solr.req.meta.client = s.client
	s.solr.req.meta.parserInfo = s.virgo.parserInfo

	s.solr.req.meta.selectionMap = make(map[string]map[string]string)

	// fill out as much as we can for a generic request

	s.solr.req.json.Params.Q = s.virgo.solrQuery
	s.solr.req.json.Params.Qt = s.pool.config.Local.Solr.Params.Qt
	s.solr.req.json.Params.DefType = s.pool.config.Local.Solr.Params.DefType
	s.solr.req.json.Params.Fl = nonemptyValues(s.pool.config.Local.Solr.Params.Fl)
	s.solr.req.json.Params.Start = restrictValue("start", s.virgo.req.Pagination.Start, 0, 0)
	s.solr.req.json.Params.Rows = restrictValue("rows", s.virgo.req.Pagination.Rows, 0, 0)

	// build fq based on global or pool context
	fq := s.pool.config.Local.Solr.Params.GlobalFq
	if s.virgo.flags.preSearchFilters == false {
		fq = append(fq, s.pool.config.Local.Solr.Params.PoolFq...)
	}
	s.solr.req.json.Params.Fq = nonemptyValues(fq)

	if s.virgo.req.Sort.SortID != "" {
		s.solr.req.json.Params.Sort = fmt.Sprintf("%s %s", s.pool.maps.definedSorts[s.virgo.req.Sort.SortID].Field, s.virgo.req.Sort.Order)
	}

	if s.virgo.flags.groupResults == true && s.virgo.flags.requestFacets == false {
		grouping := fmt.Sprintf("{!collapse field=%s}", s.pool.config.Local.Solr.GroupField)
		s.solr.req.json.Params.Fq = append(s.solr.req.json.Params.Fq, grouping)
	}

	// add facets/filters

	s.solr.req.meta.internalFacets, s.solr.req.meta.requestFacets = s.solrInternalRequestFacets()

	if s.virgo.flags.requestFacets == true && len(s.solr.req.meta.requestFacets) > 0 {
		s.solr.req.json.Facets = s.solr.req.meta.requestFacets
	}

	s.solr.req.buildFilters(s, s.virgo.req.Filters, s.solr.req.meta.internalFacets, s.pool.config.Global.Availability)

	if s.client.opts.debug == true {
		s.solr.req.json.Params.DebugQuery = "on"
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) solrSearchRequest() searchResponse {
	var err error

	var p *solrParserInfo

	// caller might have already supplied a Solr query
	if s.virgo.solrQuery == "" {
		if p, err = s.virgoQueryConvertToSolr(s.virgo.req.Query); err != nil {
			return searchResponse{status: http.StatusInternalServerError, err: fmt.Errorf("failed to convert Virgo query to Solr query: %s", err.Error())}
		}

		if p.containsUnsupportedFilters == true {
			s.virgo.skipQuery = true
		}

		s.virgo.solrQuery = p.query
		s.virgo.parserInfo = p
	}

	if resp := s.solrRequestWithDefaults(); resp.err != nil {
		return resp
	}

	return searchResponse{status: http.StatusOK}
}
