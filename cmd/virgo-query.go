package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

func (s *searchContext) virgoQueryConvertToSolr(virgoQuery string) (*solrParserInfo, error) {
	var sp solrParserInfo
	var err error
	var query string

	if query, err = v4parser.ConvertToSolrWithParser(&sp.parser, virgoQuery); err != nil {
		return nil, err
	}

	sp.query = query

	total := len(sp.parser.FieldValues)

	sp.titles = sp.parser.FieldValues["title"]
	sp.keywords = sp.parser.FieldValues["keyword"]
	sp.fulltexts = sp.parser.FieldValues["fulltext"]
	sp.identifiers = sp.parser.FieldValues["identifier"]

	sp.isSingleTitleSearch = total == 1 && len(sp.titles) == 1
	sp.isSingleKeywordSearch = total == 1 && len(sp.keywords) == 1
	sp.isSingleIdentifierSearch = total == 1 && len(sp.identifiers) == 1
	sp.isFulltextSearch = len(sp.fulltexts) > 0

	return &sp, nil
}
