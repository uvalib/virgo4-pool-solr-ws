package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

var solrAvailableFacets map[string]solrRequestFacet
var virgoAvailableFacets []string

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
	s.json.Params.Qt = config.solrParameterQt.value
}

func (s *solrRequest) buildParameterDefType() {
	s.json.Params.DefType = config.solrParameterDefType.value
}

func (s *solrRequest) buildParameterFq() {
	// leaders must be defined with beginning + or -

	fqall := []string{config.solrParameterFq.value, config.poolLeaders.value}

	s.json.Params.Fq = nonemptyValues(fqall)
}

func (s *solrRequest) buildParameterFl() {
	flall := strings.Split(config.solrParameterFl.value, ",")

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
		facets = solrAvailableFacets
	default:
		solrFacet, ok := solrAvailableFacets[facet]

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
		solrFacet, ok := solrAvailableFacets[filter.Name]

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
	s.json.Params.GroupField = "work_title2_key_sort"
	s.json.Params.GroupLimit = 10000
	s.json.Params.GroupMain = false
//	s.json.Params.GroupNGroups = true
	s.json.Params.Group = true
}

func solrRequestWithDefaults(v VirgoSearchRequest) solrRequest {
	var s solrRequest

	s.meta.client = v.meta.client

	// fill out as much as we can for a generic request
	s.buildParameterQ(v.meta.solrQuery)
	s.buildParameterQt()
	s.buildParameterDefType()
	s.buildParameterFq()
	s.buildParameterFl()

	s.buildParameterStart(v.Pagination.Start)
	s.buildParameterRows(v.Pagination.Rows)

	if v.meta.requestFacets == true {
		s.buildFacets(v.Facet)
	}

	s.buildFilters(v.Filters)

	s.buildGrouping()

	return s
}

func solrSearchRequest(v VirgoSearchRequest) (*solrRequest, error) {
	var err error

	var p *solrParserInfo

	// caller might have already supplied a Solr query
	if v.meta.solrQuery == "" {
		if p, err = virgoQueryConvertToSolr(v.Query); err != nil {
			return nil, fmt.Errorf("Virgo query to Solr conversion error: %s", err.Error())
		}

		v.meta.solrQuery = p.query
	}

	solrReq := solrRequestWithDefaults(v)

	solrReq.meta.parserInfo = p

	return &solrReq, nil
}

func solrRecordRequest(v VirgoSearchRequest) (*solrRequest, error) {
	solrReq := solrRequestWithDefaults(v)

	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	solrReq.json.Params.Start = 0
	solrReq.json.Params.Rows = 2

	return &solrReq, nil
}

func init() {
	type facetInfo struct {
		Facets []solrRequestFacet `json:"facets"`
	}

	var facets facetInfo

	if err := json.Unmarshal([]byte(config.solrAvailableFacets.value), &facets); err != nil {
		log.Printf("error parsing available facets json: %s", err.Error())
		os.Exit(1)
	}

	solrAvailableFacets = make(map[string]solrRequestFacet)

	for _, facet := range facets.Facets {
		virgoAvailableFacets = append(virgoAvailableFacets, facet.Name)
		solrAvailableFacets[facet.Name] = solrRequestFacet{Type: facet.Type, Field: facet.Field, Sort: facet.Sort}
	}
}
