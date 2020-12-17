package main

import (
	"strings"

	"github.com/uvalib/virgo4-parser/v4parser"
)

func (s *searchContext) virgoQueryConvertToSolr(virgoQuery string) (*solrParserInfo, error) {
	var sp solrParserInfo
	var err error
	var query string

	if query, err = v4parser.ConvertToSolrWithParserAndTimeout(&sp.parser, virgoQuery, 10); err != nil {
		return nil, err
	}

	// validate any filter IDs, and convert them to solr fields

	for _, filterClause := range sp.parser.FieldValues["filter"] {
		filterParts := strings.Split(filterClause, ":")

		// first, verify this is a supported filter
		filterID := filterParts[0]

		filter, ok := s.pool.maps.definedFilters[filterID]

		if ok == false {
			s.log("query contains unsupported filter ID: [%s]", filterID)
			sp.containsUnsupportedFilters = true
			continue
		}

		// next, extract the filter value
		filterValue := strings.Join(filterParts[1:], ":")

		oldFragment := filterID + ":" + filterValue

		// filter values with colons will be quoted (otherwise v4 parser will reject them).
		// parse out actual value from any quoting.  the format will be:
		//   ` \"filter_value\"`
		// quoted values without colons will also be in this format, so it works in any case.

		quote := `\"`

		filterValuePrefix := ""
		filterValueSuffix := ""
		filterValueSeparator := ""
		filterValueActual := filterValue
		filterValueParts := strings.Split(filterValue, quote)

		if len(filterValueParts) > 2 {
			filterValueSeparator = quote

			filterValuePrefix = filterValueParts[0]
			filterValueParts = filterValueParts[1:]

			filterValueSuffix = filterValueParts[len(filterValueParts)-1]
			filterValueParts = filterValueParts[:len(filterValueParts)-1]

			filterValueActual = strings.Join(filterValueParts, quote)
		}

		// restrict to known values
		filterValueActual, err = s.getInternalSolrValue(filter.Solr.Field, filterValueActual)
		if err != nil {
			s.warn("query contains filter ID [%s] with unsupported value: [%s]", filterID, filterValueActual)
			sp.containsUnsupportedFilters = true
			continue
		}

		filterValue = strings.Join([]string{filterValuePrefix, filterValueActual, filterValueSuffix}, filterValueSeparator)

		newFragment := filter.Solr.Field + ":" + filterValue

		oldQuery := query
		newQuery := strings.ReplaceAll(query, oldFragment, newFragment)

		// make sure the replacement happened
		if newQuery == oldQuery {
			s.err("failed to rewrite query fragment:")
			s.err("old query fragment: [%s]", oldFragment)
			s.err("new query fragment: [%s]", newFragment)
			s.err("old query complete: [%s]", oldQuery)
			s.err("new query complete: [%s]", newQuery)
			sp.containsUnsupportedFilters = true
			continue
		}

		query = newQuery
	}

	sp.query = query

	// do some pre-analysis

	// just checking for single-term searches so this approach is sufficient:

	total := len(sp.parser.FieldValues)

	sp.titles = sp.parser.FieldValues["title"]
	sp.keywords = sp.parser.FieldValues["keyword"]

	sp.isSingleTitleSearch = total == 1 && len(sp.titles) == 1
	sp.isSingleKeywordSearch = total == 1 && len(sp.keywords) == 1

	return &sp, nil
}
