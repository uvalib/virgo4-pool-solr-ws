package main

import (
	"fmt"
	"strings"
)

// functions that map virgo data into solr data

func (s *solrRequest) restrictValue(field string, val int, min int, fallback int) int {
	// default, if requested value isn't large enough
	res := fallback

	if val >= min {
		res = val
	} else {
		warning := fmt.Sprintf(`value for "%s" is less than the minimum allowed value %d; defaulting to %d`, field, min, fallback)
		s.meta.client.log(warning)
		s.meta.warnings = append(s.meta.warnings, warning)
	}

	return res
}

func nonemptyValues(val []string) []string {
	res := []string{}

	for _, s := range val {
		if s != "" {
			res = append(res, s)
		}
	}

	return res
}

func (s *solrRequest) buildParameterQ(query string) {
	s.json.Params.Q = query
}

func (s *solrRequest) buildParameterStart(n int) {
	s.json.Params.Start = s.restrictValue("start", n, minimumStart, defaultStart)
}

func (s *solrRequest) buildParameterRows(n int) {
	s.json.Params.Rows = s.restrictValue("rows", n, minimumRows, defaultRows)
}

func (s *solrRequest) buildParameterQt() {
	s.json.Params.Qt = pool.config.solrParameterQt
}

func (s *solrRequest) buildParameterDefType() {
	s.json.Params.DefType = pool.config.solrParameterDefType
}

func (s *solrRequest) buildParameterFq() {
	// leaders must be defined with beginning + or -

	fqall := []string{pool.config.solrParameterFq, pool.config.poolLeaders}

	s.json.Params.Fq = nonemptyValues(fqall)
}

func (s *solrRequest) buildParameterFl() {
	flall := strings.Split(pool.config.solrParameterFl, ",")

	s.json.Params.Fl = nonemptyValues(flall)
}

func (s *solrRequest) buildFacets(facet string) {
	if facet == "" {
		s.meta.advertiseFacets = true
		return
	}

	facets := make(map[string]solrRequestFacet)

	switch facet {
	case "all":
		facets = pool.solr.availableFacets
	default:
		solrFacet, ok := pool.solr.availableFacets[facet]

		if ok == false {
			warning := fmt.Sprintf("ignoring unrecognized facet: [%s]", facet)
			s.meta.client.log(warning)
			s.meta.warnings = append(s.meta.warnings, warning)
			s.meta.advertiseFacets = true
		} else {
			facets[facet] = solrFacet
		}
	}

	if len(facets) > 0 {
		s.json.Facets = facets
	}
}

func (s *solrRequest) buildFilters(filters *[]VirgoFilter) {
	if filters == nil {
		return
	}

	for _, filter := range *filters {
		solrFacet, ok := pool.solr.availableFacets[filter.Name]

		if ok == false {
			warning := fmt.Sprintf("ignoring unrecognized filter: [%s]", filter.Name)
			s.meta.client.log(warning)
			s.meta.warnings = append(s.meta.warnings, warning)
			continue
		}

		solrFilter := fmt.Sprintf(`%s:"%s"`, solrFacet.Field, filter.Value)

		s.json.Params.Fq = append(s.json.Params.Fq, solrFilter)
	}
}

func (s *solrRequest) buildGrouping() {
	s.json.Params.GroupField = pool.config.solrGroupField
	s.json.Params.GroupLimit = 10000
	s.json.Params.GroupMain = false
	//s.json.Params.GroupNGroups = true
	s.json.Params.Group = true
}

func (s *searchContext) solrRequestWithDefaults() {
	var solrReq solrRequest

	solrReq.meta.client = s.virgoReq.meta.client

	// fill out as much as we can for a generic request
	solrReq.buildParameterQ(s.virgoReq.meta.solrQuery)
	solrReq.buildParameterQt()
	solrReq.buildParameterDefType()
	solrReq.buildParameterFq()
	solrReq.buildParameterFl()

	solrReq.buildParameterStart(s.virgoReq.Pagination.Start)
	solrReq.buildParameterRows(s.virgoReq.Pagination.Rows)

	if s.virgoReq.meta.requestFacets == true {
		solrReq.buildFacets(s.virgoReq.Facet)
	}

	solrReq.buildFilters(s.virgoReq.Filters)

	if s.client.grouped == true {
		solrReq.buildGrouping()
	}

	s.solrReq = &solrReq
}

func (s *searchContext) solrSearchRequest() error {
	var err error

	var p *solrParserInfo

	// caller might have already supplied a Solr query
	if s.virgoReq.meta.solrQuery == "" {
		if p, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
			return fmt.Errorf("Virgo query to Solr conversion error: %s", err.Error())
		}

		s.virgoReq.meta.solrQuery = p.query
	}

	s.solrRequestWithDefaults()

	s.solrReq.meta.parserInfo = p

	return nil
}

func (s *searchContext) solrRecordRequest() error {
	s.solrRequestWithDefaults()

	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	s.solrReq.json.Params.Start = 0
	s.solrReq.json.Params.Rows = 2

	return nil
}
