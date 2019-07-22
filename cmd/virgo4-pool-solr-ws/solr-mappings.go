package main

import (
	"fmt"
	"strings"
)

var solrAvailableFacets map[string]solrRequestFacet

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
	s.json.Params.Start = s.restrictValue("start", n, 0, 0)
}

func (s *solrRequest) buildParameterRows(n int) {
	s.json.Params.Rows = s.restrictValue("rows", n, 1, 10)
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

func (s *solrRequest) buildFacets(facets *VirgoFacetList) {
	if facets == nil {
		return
	}

	// special case "all" returns all supported facets with default offset/limit/sort
	if len(*facets) == 1 && (*facets)[0].Name == "all" {
		s.json.Facets = solrAvailableFacets
		return
	}

	// otherwise, ensure client is requesting valid fields, and use its desired offset/limit/sort values
	s.json.Facets = make(map[string]solrRequestFacet)

	for _, facet := range *facets {
		solrFacet, ok := solrAvailableFacets[facet.Name]

		if ok == false {
			warning := fmt.Sprintf("ignoring unrecognized facet field: [%s]", facet.Name)
			s.meta.client.log(warning)
			s.meta.warnings = append(s.meta.warnings, warning)
			continue
		}

		// update with provided values, if any

		// safe to just overwrite, as they will only be non-zero if client specifies it
		solrFacet.Offset = facet.Offset
		solrFacet.Limit = facet.Limit

		// need to check before overwriting
		if facet.Sort != "" {
			solrFacet.Sort = facet.Sort
		}

		s.json.Facets[facet.Name] = solrFacet
	}
}

func (s *solrRequest) buildFilters(filters *VirgoFacetList) {
	s.json.Params.Fq = []string{}

	if filters == nil {
		return
	}

	for _, filter := range *filters {
		solrFacet, ok := solrAvailableFacets[filter.Name]

		if ok == false {
			warning := fmt.Sprintf("ignoring unrecognized filter field: [%s]", filter.Name)
			s.meta.client.log(warning)
			s.meta.warnings = append(s.meta.warnings, warning)
			continue
		}

		solrFilter := fmt.Sprintf(`%s:"%s"`, solrFacet.Field, filter.Value)

		s.json.Params.Fq = append(s.json.Params.Fq, solrFilter)
	}
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

	if v.Pagination != nil {
		s.buildParameterStart(v.Pagination.Start)
		s.buildParameterRows(v.Pagination.Rows)
	}

	if v.meta.actualSearch == true {
		s.buildFacets(v.Facets)
	}

	s.buildFilters(v.Filters)

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
	solrAvailableFacets = make(map[string]solrRequestFacet)

	solrAvailableFacets["authors"] = solrRequestFacet{Type: "terms", Field: "author_facet_f", Sort: "index"}
	solrAvailableFacets["subjects"] = solrRequestFacet{Type: "terms", Field: "subject_f", Sort: "count"}
	solrAvailableFacets["languages"] = solrRequestFacet{Type: "terms", Field: "language_f", Sort: "count"}
	solrAvailableFacets["libraries"] = solrRequestFacet{Type: "terms", Field: "library_f", Sort: "count"}
	solrAvailableFacets["call_numbers_broad"] = solrRequestFacet{Type: "terms", Field: "call_number_broad_f", Sort: "index"}
	solrAvailableFacets["call_numbers_narrow"] = solrRequestFacet{Type: "terms", Field: "call_number_narrow_f", Sort: "index"}
}
