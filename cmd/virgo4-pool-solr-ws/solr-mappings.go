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

func (s *solrRequest) buildParameterQt(qt string) {
	s.json.Params.Qt = qt
}

func (s *solrRequest) buildParameterDefType(defType string) {
	s.json.Params.DefType = defType
}

func (s *solrRequest) buildParameterFq(fq string, poolDefinition string) {
	fqall := []string{fq, poolDefinition}

	s.json.Params.Fq = nonemptyValues(fqall)
}

func (s *solrRequest) buildParameterFl(fl string) {
	flall := strings.Split(fl, ",")

	s.json.Params.Fl = nonemptyValues(flall)
}

func (s *solrRequest) buildFacets(availableFacets map[string]solrRequestFacet) {
	if len(availableFacets) > 0 {
		s.json.Facets = availableFacets
	}
}

func (s *solrRequest) buildFilters(filterGroups *[]VirgoFilter, availableFacets map[string]solrRequestFacet) {
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

		solrFilter := fmt.Sprintf(`%s:"%s"`, solrFacet.Field, filter.Value)

		s.json.Params.Fq = append(s.json.Params.Fq, solrFilter)

		// add this filter to selection map
		if s.meta.selectionMap[filter.FacetID] == nil {
			s.meta.selectionMap[filter.FacetID] = make(map[string]bool)
		}

		s.meta.selectionMap[filter.FacetID][filter.Value] = true
	}
}

func (s *solrRequest) buildGrouping(groupField string) {
	// groups take 2:
	grouping := fmt.Sprintf("{!collapse field=%s}", groupField)
	s.json.Params.Fq = append(s.json.Params.Fq, grouping)
}

func (s *searchContext) solrAvailableFacets() map[string]solrRequestFacet {
	// build customized/personalized available facets from facets definition

	availableFacets := make(map[string]solrRequestFacet)

	for _, facet := range s.pool.solr.availableFacets {
		f := solrRequestFacet{
			Type:          facet.Type,
			Field:         facet.Field,
			Sort:          facet.Sort,
			Offset:        facet.Offset,
			Limit:         facet.Limit,
			exposedValues: facet.ExposedValues,
		}

		if facet.FieldAuth != "" {
			if s.virgoReq.meta.client.isAuthenticated() == true {
				f.Field = facet.FieldAuth
				s.log("[FACET] authenticated session using facet: [%s]", f.Field)
			} else {
				s.log("[FACET] unauthenticated session using facet: [%s]", f.Field)
			}
		}

		availableFacets[facet.Name] = f
	}

	return availableFacets
}

func (s *searchContext) solrRequestWithDefaults() {
	var solrReq solrRequest

	solrReq.meta.client = s.virgoReq.meta.client
	solrReq.meta.parserInfo = s.virgoReq.meta.parserInfo

	solrReq.meta.selectionMap = make(map[string]map[string]bool)

	// fill out as much as we can for a generic request
	solrReq.buildParameterQ(s.virgoReq.meta.solrQuery)
	solrReq.buildParameterQt(s.pool.config.solrParameterQt)
	solrReq.buildParameterDefType(s.pool.config.solrParameterDefType)
	solrReq.buildParameterFq(s.pool.config.solrParameterFq, s.pool.config.poolDefinition)
	solrReq.buildParameterFl(s.pool.config.solrParameterFl)

	solrReq.buildParameterStart(s.virgoReq.Pagination.Start)
	solrReq.buildParameterRows(s.virgoReq.Pagination.Rows)

	// add facets/filters

	availableFacets := s.solrAvailableFacets()

	if s.virgoReq.meta.requestFacets == true {
		solrReq.buildFacets(availableFacets)
	}

	solrReq.buildFilters(s.virgoReq.Filters, availableFacets)

	if s.client.opts.grouped == true {
		solrReq.buildGrouping(s.pool.config.solrGroupField)
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
		s.virgoReq.meta.parserInfo = p
	}

	s.solrRequestWithDefaults()

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
