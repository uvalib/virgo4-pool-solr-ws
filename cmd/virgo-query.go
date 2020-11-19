package main

import (
	"errors"
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
		filterID := strings.Split(filterClause, ":")[0]
		filter, ok := s.pool.maps.filters[filterID]

		if ok == false {
			s.log("abandoning query conversion due to unsupported filter ID: [%s]", filterID)
			return nil, errors.New("query contains unsupported filter ID")
		}

		query = strings.ReplaceAll(query, filterID+":", filter.Solr.Field+":")
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
