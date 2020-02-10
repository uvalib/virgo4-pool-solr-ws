package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultStart = 0
const minimumStart = 0

const defaultRows = 0
const minimumRows = 0

type searchContext struct {
	pool           *poolContext
	client         *clientContext
	virgoReq       VirgoSearchRequest
	virgoPoolRes   *VirgoPoolResult
	virgoRecordRes *VirgoRecord
	solrReq        *solrRequest
	solrRes        *solrResponse
	confidence     string
}

func confidenceIndex(s string) int {
	// invalid values will be 0 (less than "low")
	return map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
		"exact":  4,
	}[s]
}

func (s *searchContext) init(p *poolContext, c *clientContext) {
	s.pool = p

	s.client = c

	s.virgoReq.meta.client = s.client
	s.virgoReq.Pagination.Start = defaultStart
	s.virgoReq.Pagination.Rows = defaultRows
}

func (s *searchContext) copySearchContext() *searchContext {
	// performs a copy somewhere between shallow and deep
	// (just enough to let this context be used for another search
	// without potentially clobbering the original context by leaving
	// pointers into the original context)

	sc := &searchContext{}

	sc.pool = s.pool

	// copy client (modified for speculative searches)
	c := *s.client
	sc.client = &c

	v := s.virgoReq
	sc.virgoReq = v

	return sc
}

func (s *searchContext) log(format string, args ...interface{}) {
	s.client.log(format, args...)
}

func (s *searchContext) err(format string, args ...interface{}) {
	s.client.err(format, args...)
}

func (s *searchContext) performQuery() error {
	var err error

	if err = s.solrSearchRequest(); err != nil {
		s.err("query creation error: %s", err.Error())
		return err
	}

	if err = s.solrQuery(); err != nil {
		s.err("query execution error: %s", err.Error())
		return err
	}

	return nil
}

func (s *searchContext) getPoolQueryResults() error {
	var err error

	if err = s.performQuery(); err != nil {
		return err
	}

	if err = s.virgoSearchResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return err
	}

	s.confidence = s.virgoPoolRes.Confidence

	return nil
}

func (s *searchContext) getRecordQueryResults() error {
	var err error

	if err = s.performQuery(); err != nil {
		return err
	}

	if err = s.virgoRecordResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return err
	}

	return nil
}

func (s *searchContext) newSearchWithTopResult(query string) (*searchContext, error) {
	// returns a new search context with the top result of the supplied query
	top := s.copySearchContext()

	// just want first result, not first result group
	top.client.opts.grouped = false

	top.virgoReq.Query = query
	top.virgoReq.meta.solrQuery = ""
	top.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 1}

	if err := top.getPoolQueryResults(); err != nil {
		return nil, err
	}

	return top, nil
}

type byConfidence struct {
	results []*searchContext
}

func (b *byConfidence) Len() int {
	return len(b.results)
}

func (b *byConfidence) Swap(i, j int) {
	b.results[i], b.results[j] = b.results[j], b.results[i]
}

func (b *byConfidence) Less(i, j int) bool {
	// sort by confidence index

	if confidenceIndex(b.results[i].virgoPoolRes.Confidence) < confidenceIndex(b.results[j].virgoPoolRes.Confidence) {
		return false
	}

	if confidenceIndex(b.results[i].virgoPoolRes.Confidence) > confidenceIndex(b.results[j].virgoPoolRes.Confidence) {
		return true
	}

	// confidence is equal; sort by score

	return b.results[i].solrRes.meta.maxScore > b.results[j].solrRes.meta.maxScore
}

func (s *searchContext) performSpeculativeTitleSearch() (*searchContext, error) {
	// if the original query will include the top result, no need to check it here,
	// as the original query will determine the correct confidence level when it runs.
	// otherwise, query the top result so that we can use its potentially better/more
	// accurate confidence level in the original query's results.

	if s.virgoReq.Pagination.Start != 0 {
		s.log("TITLE SEARCH: determining true confidence level")

		return s.newSearchWithTopResult(s.virgoReq.Query)
	}

	return s, nil
}

func (s *searchContext) performSpeculativeKeywordSearch(origSearchTerm string) (*searchContext, error) {
	var err error
	var title, author, keyword *searchContext
	var searchResults []*searchContext

	// if the client specified no intuition, just return original query
	if s.client.opts.intuit != true {
		return s, nil
	}

	// first, unescape the search term (which may have been escaped by the query parser)

	searchTerm := origSearchTerm

	var unquotedSearchTerm string
	if unquotedSearchTerm, err = strconv.Unquote(fmt.Sprintf(`"%s"`, origSearchTerm)); err == nil {
		searchTerm = unquotedSearchTerm
	}

	s.log("[INTUIT] KEYWORD SEARCH: intuiting best search for [%s]", searchTerm)

	s.log("[INTUIT] checking if keyword search term [%s] might be a title or author search...", searchTerm)

	if keyword, err = s.newSearchWithTopResult(s.virgoReq.Query); err != nil {
		return nil, err
	}

	s.log("[INTUIT] keyword: confidence = [%s]  maxScore = [%0.2f]", keyword.virgoPoolRes.Confidence, keyword.solrRes.meta.maxScore)
	searchResults = append(searchResults, keyword)

	if title, err = keyword.newSearchWithTopResult(fmt.Sprintf("title:{%s}", searchTerm)); err == nil {
		s.log("[INTUIT] title: confidence = [%s]  maxScore = [%0.2f]", title.virgoPoolRes.Confidence, title.solrRes.meta.maxScore)
		searchResults = append(searchResults, title)
	}

	if author, err = keyword.newSearchWithTopResult(fmt.Sprintf("author:{%s}", searchTerm)); err == nil {
		s.log("[INTUIT] author: confidence = [%s]  maxScore = [%0.2f]", author.virgoPoolRes.Confidence, author.solrRes.meta.maxScore)
		searchResults = append(searchResults, author)
	}

	confidenceSort := byConfidence{results: searchResults}
	sort.Sort(&confidenceSort)

	return confidenceSort.results[0], nil
}

func (s *searchContext) performSpeculativeSearches() (*searchContext, error) {
	var err error
	var parsedQuery *solrParserInfo

	// maybe facet buckets might differ based on speculative search?
	/*
		// facet-only requests don't need speculation, as the client is only looking for buckets
		if s.virgoReq.Pagination.Rows == 0 {
			return s, nil
		}
	*/

	// parse original query to determine query type

	if parsedQuery, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
		return nil, err
	}

	// single-term title-only search special handling

	if parsedQuery.isSingleTitleSearch == true {
		return s.performSpeculativeTitleSearch()
	}

	// single-term keyword search special handling

	if parsedQuery.isSingleKeywordSearch == true {
		return s.performSpeculativeKeywordSearch(firstElementOf(parsedQuery.keywords))
	}

	// fallthrough: just return original query

	return s, nil
}

func (s *searchContext) newSearchWithRecordListForGroups(groups []string) (*searchContext, error) {
	// NOTE: groups passed in are quoted strings

	c := s.copySearchContext()

	// just want records
	c.client.opts.grouped = false

	c.virgoReq.meta.solrQuery = fmt.Sprintf(`%s AND %s:(%s)`, c.virgoReq.meta.solrQuery, s.pool.config.solrGroupField, strings.Join(groups, " OR "))
	c.virgoReq.meta.requestFacets = false

	// get everything!  even bible (5000+)
	c.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 100000}

	if err := c.getPoolQueryResults(); err != nil {
		return nil, err
	}

	return c, nil
}

func (s *searchContext) populateGroups() error {
	// populate record list for each group (i.e. entry in initial record list)
	// by querying for all group records in one request, and sorting the results to the correct groups

	if s.client.opts.grouped == false || s.solrRes.meta.numGroups == 0 {
		return nil
	}

	// no need to group facet endpoint results
	if s.virgoReq.meta.requestFacets == true {
		return nil
	}

	var groups VirgoGroups

	var groupValues []string

	groupValueMap := make(map[string]int)

	for i, groupRecord := range s.solrRes.Response.Docs {
		groupValue := s.getSolrGroupFieldValue(&groupRecord)
		groupValues = append(groupValues, fmt.Sprintf(`"%s"`, groupValue))
		groupValueMap[groupValue] = i
		var records VirgoRecords
		groups = append(groups, VirgoGroup{Value: groupValue, RecordList: records})
	}

	r, err := s.newSearchWithRecordListForGroups(groupValues)
	if err != nil {
		return err
	}

	// loop through records to route to correct group

	start := time.Now()
	for _, record := range *r.virgoPoolRes.RecordList {
		v := groupValueMap[record.groupValue]
		groups[v].RecordList = append(groups[v].RecordList, record)
	}
	s.log("[GROUP] map groups: %5d ms", int64(time.Since(start)/time.Millisecond))

	// loop through groups to assign count and fields

	start = time.Now()
	for i := range groups {
		group := &groups[i]

		group.Count = len(group.RecordList)

		// FIXME: old way (client keys off of field types to generate labels from first record)

		parts := strings.Split(group.Value, "/")

		title := parts[0]
		author := parts[1]
		format := parts[2]

		if title != "" {
			group.addBasicField(newField("title", s.client.localize("FieldTitle"), title).setType("title"))
		}

		if author != "" {
			group.addBasicField(newField("author", s.client.localize(s.pool.config.solrAuthorLabel), author).setType("author"))
		}

		if format != "" {
			group.addBasicField(newField("format", s.client.localize("FieldFormat"), format))
		}

		// cover image url
		// just use url from first grouped result that has one
		// (they all will for now, but we check properly anyway)

		gotCover := false

		for _, r := range group.RecordList {
			for _, f := range r.Fields {
				if f.Name == "cover_image" {
					group.addBasicField(&f)
					gotCover = true
					break
				}
			}

			if gotCover == true {
				break
			}
		}

		/*
			// FIXME: new way (client uses these fields as-is)

			// set most group fields based on first result

			for _, f := range group.RecordList[0].Fields {
				switch f.Name {
				case "title":
					fallthrough
				case "subtitle":
					fallthrough
				case "author":
					fallthrough
				case "cover_image":
					group.addBasicField(&f)
				}
			}

			// set group format based on solr grouping field
			// (or intersection of all records?)

			parts := strings.Split(group.Value, "/")

			if len(parts) >= 2 {
				format := strings.Title(parts[2])

				if format != "" {
					group.addBasicField(newField("format", s.client.localize("FieldFormat"), format))
				}
			}
		*/
	}
	s.log("[GROUP] group vals: %5d ms", int64(time.Since(start)/time.Millisecond))

	s.virgoPoolRes.GroupList = &groups

	// remove record list, as results are now in groups
	s.virgoPoolRes.RecordList = nil

	return nil
}

func (s *searchContext) validateSearchRequest() error {
	// quick validations we can do up front

	// ensure we received either zero or one filter group,
	// and that any filters provided are supported

	if s.virgoReq.Filters != nil {
		numFilterGroups := len(*s.virgoReq.Filters)

		switch {
		case numFilterGroups > 1:
			return errors.New("received too many filter groups")

		case numFilterGroups == 1:
			availableFacets := s.solrAvailableFacets()

			filterGroup := (*s.virgoReq.Filters)[0]

			s.log("received filter group: [%s]", filterGroup.PoolID)

			for _, filter := range filterGroup.Facets {
				if _, ok := availableFacets[filter.FacetID]; ok == false {
					return fmt.Errorf("received unrecognized filter: [%s]", filter.FacetID)
				}
			}
		}
	}

	return nil
}

func (s *searchContext) handleSearchOrFacetsRequest() error {
	var err error
	var top *searchContext

	if err = s.validateSearchRequest(); err != nil {
		return err
	}

	// save original facet request flag
	requestFacets := s.virgoReq.meta.requestFacets

	if top, err = s.performSpeculativeSearches(); err != nil {
		return err
	}

	// use query syntax from chosen search
	s.virgoReq.Query = top.virgoReq.Query

	// restore original facet request flag
	s.virgoReq.meta.requestFacets = requestFacets

	// now do the search
	if err = s.getPoolQueryResults(); err != nil {
		return err
	}

	// populate group list, if this is a grouped request
	if err = s.populateGroups(); err != nil {
		return err
	}

	// restore actual confidence
	if confidenceIndex(top.confidence) > confidenceIndex(s.virgoPoolRes.Confidence) {
		s.log("overriding confidence [%s] with [%s]", s.virgoPoolRes.Confidence, top.confidence)
		s.virgoPoolRes.Confidence = top.confidence
	}

	s.virgoPoolRes.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	return nil
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	if err := s.handleSearchOrFacetsRequest(); err != nil {
		return nil, err
	}

	s.virgoPoolRes.FacetList = nil
	return s.virgoPoolRes, nil
}

func (s *searchContext) handleFacetsRequest() (*VirgoFacetsResult, error) {
	// override these values from the original search query, since we are
	// only interested in facets, not records
	s.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 0}

	s.virgoReq.meta.requestFacets = true

	if err := s.handleSearchOrFacetsRequest(); err != nil {
		return nil, err
	}

	virgoFacetsRes := VirgoFacetsResult{
		FacetList: s.virgoPoolRes.FacetList,
		ElapsedMS: s.virgoPoolRes.ElapsedMS,
	}

	return &virgoFacetsRes, nil
}

func (s *searchContext) handleRecordRequest() (*VirgoRecord, error) {
	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	s.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 2}

	if err := s.getRecordQueryResults(); err != nil {
		return nil, err
	}

	return s.virgoRecordRes, nil
}

func (s *searchContext) handlePingRequest() error {
	// override these values from defaults.  we are not interested
	// in records, just connectivity
	s.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 0}

	if err := s.performQuery(); err != nil {
		return err
	}

	return nil
}
