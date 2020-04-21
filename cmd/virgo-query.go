package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

func virgoQueryConvertToSolr(virgoQuery string) (*solrParserInfo, error) {
	var sp solrParserInfo
	var err error

	if sp.query, err = v4parser.ConvertToSolrWithParser(&sp.parser, virgoQuery); err != nil {
		return nil, err
	}

	// do some pre-analysis

	// just checking for single-term searches so this approach is sufficient:

	total := len(sp.parser.FieldValues)

	sp.titles = sp.parser.FieldValues["title"]
	sp.keywords = sp.parser.FieldValues["keyword"]

	sp.isSingleTitleSearch = total == 1 && len(sp.titles) == 1
	sp.isSingleKeywordSearch = total == 1 && len(sp.keywords) == 1

	return &sp, nil
}
