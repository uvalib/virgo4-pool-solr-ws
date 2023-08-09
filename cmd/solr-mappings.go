package main

import (
	"fmt"
	"net/http"
	"strings"

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

		// if this is not the facet cache requesting all facets, then
		// omit this selected filter if it depends on other filters, none of which are selected
		dependentFilterXIDs := ctx.resourceTypeCtx.FilterOverrides[filter.FacetID].DependentFilterXIDs

		if ctx.virgo.flags.facetCache == false && len(dependentFilterXIDs) > 0 {
			numSelected := 0

			for _, facet := range dependentFilterXIDs {
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
				availabilityFacet := availability.FilterConfig.FieldAnon
				if s.meta.client.isAuthenticated() == true {
					availabilityFacet = availability.FilterConfig.FieldAuth
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

		// add this filter to selection map
		if s.meta.selectionMap[filter.FacetID] == nil {
			s.meta.selectionMap[filter.FacetID] = make(map[string]string)
		}

		s.meta.selectionMap[filter.FacetID][filterValue] = solrFilter
	}

	// build filter query based on OR'd filter values among AND'd filter types

	var orFilters []string

	for filterID, selectedValues := range s.meta.selectionMap {
		// when iterating over facets, do not include current facet in filter queries
		// so that all possible matching values for this facet are returned
		if ctx.virgo.flags.facetCache == false && ctx.virgo.flags.requestFacets == true && filterID == ctx.virgo.currentFacet {
			continue
		}

		var idFilters []string
		for _, solrFilter := range selectedValues {
			idFilters = append(idFilters, fmt.Sprintf("(%s)", solrFilter))
		}

		orFilter := strings.Join(idFilters, " OR ")
		s.meta.client.log("FILTER: applying filter: %-20s : %s", filterID, orFilter)

		orFilters = append(orFilters, orFilter)
	}

	s.json.Params.Fq = append(s.json.Params.Fq, orFilters...)
}

func (s *searchContext) solrInternalRequestFacets() (map[string]*solrRequestFacet, map[string]*solrRequestFacet) {
	// build customized/personalized available facets from facets definition

	internalFacets := make(map[string]*solrRequestFacet)
	requestFacets := make(map[string]*solrRequestFacet)

	auth := s.client.isAuthenticated()

	// should we request facets or pre-search filters?

	var sourceFacets map[string]*poolConfigFilter
	if s.virgo.flags.facetCache == true {
		if s.virgo.flags.globalFacetCache == true {
			sourceFacets = s.pool.maps.preSearchFilters
		} else {
			sourceFacets = s.pool.maps.supportedFilters
		}
	} else {
		sourceFacets = s.resourceTypeCtx.filterMap
	}

	for xid := range sourceFacets {
		facet := sourceFacets[xid]

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

		// when iterating over facets, only include current facet in solr request
		if s.virgo.flags.facetCache == false && s.virgo.flags.requestFacets == true && xid != s.virgo.currentFacet {
			continue
		}

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
	fq := s.pool.config.Local.Solr.Params.Fq.Global

	if s.virgo.flags.includeVisible == true {
		fq = append(fq, s.pool.config.Local.Solr.Params.Fq.Visible...)
	}

	if s.virgo.flags.includeHidden == true {
		fq = append(fq, s.pool.config.Local.Solr.Params.Fq.Hidden...)
	}

	if s.virgo.flags.globalFacetCache == false {
		fq = append(fq, s.pool.config.Local.Solr.Params.Fq.Pool...)
	}

	if s.virgo.flags.groupResults == true && s.virgo.flags.requestFacets == false {
		grouping := fmt.Sprintf("{!collapse field=%s}", s.pool.config.Local.Solr.GroupField)
		fq = append(fq, grouping)
	}

	s.solr.req.json.Params.Fq = nonemptyValues(fq)

	// set sort options
	if s.virgo.req.Sort.SortID != "" {
		s.solr.req.json.Params.Sort = fmt.Sprintf("%s %s", s.pool.maps.definedSorts[s.virgo.req.Sort.SortID].Field, s.virgo.req.Sort.Order)
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

	// set up highlighting
	s.solr.req.json.Params.Hl = "false"
	if s.virgo.flags.includeSnippets == true {
		s.solr.req.json.Params.Hl = "true"
		s.solr.req.json.Params.HlMethod = s.pool.config.Local.Solr.Highlighting.Method
		s.solr.req.json.Params.HlFl = s.pool.config.Local.Solr.Highlighting.Fl
		s.solr.req.json.Params.HlSnippets = s.pool.config.Local.Solr.Highlighting.Snippets
		s.solr.req.json.Params.HlFragsize = s.pool.config.Local.Solr.Highlighting.Fragsize
		s.solr.req.json.Params.HlFragsizeIsMinimum = s.pool.config.Local.Solr.Highlighting.FragsizeIsMinimum
		s.solr.req.json.Params.HlFragAlignRatio = s.pool.config.Local.Solr.Highlighting.FragAlignRatio
		s.solr.req.json.Params.HlMaxAnalyzedChars = s.pool.config.Local.Solr.Highlighting.MaxAnalyzedChars
		s.solr.req.json.Params.HlMultiTermQuery = s.pool.config.Local.Solr.Highlighting.MultiTermQuery
		s.solr.req.json.Params.HlTagPre = s.pool.config.Local.Solr.Highlighting.TagPre
		s.solr.req.json.Params.HlTagPost = s.pool.config.Local.Solr.Highlighting.TagPost

		// don't need all doc fields for highlight searches
		s.solr.req.json.Params.Fl = []string{s.pool.config.Local.Solr.IdentifierField}
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) populateSolrQuery() searchResponse {
	p, err := s.virgoQueryConvertToSolr(s.virgo.req.Query)

	if err != nil {
		return searchResponse{status: http.StatusBadRequest, err: fmt.Errorf("failed to convert Virgo query to Solr query: %s", err.Error())}
	}

	if p.containsUnsupportedFilters == true {
		s.virgo.skipQuery = true
	}

	s.virgo.solrQuery = p.query
	s.virgo.parserInfo = p

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) solrSearchRequest() searchResponse {
	var resp searchResponse

	s.solr.req = solrRequest{}

	// caller might have already supplied a Solr query
	if s.virgo.solrQuery == "" {
		if resp = s.populateSolrQuery(); resp.err != nil {
			return resp
		}
	}

	if resp = s.solrRequestWithDefaults(); resp.err != nil {
		return resp
	}

	return searchResponse{status: http.StatusOK}
}
