package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

func virgoQueryValidate(virgoQuery string) (bool, string) {
	return v4parser.Validate(virgoQuery)
}

func virgoQueryConvertToSolr(virgoQuery string) (*solrParserInfo, error) {
	var sp solrParserInfo
	var err error

	if sp.query, err = v4parser.ConvertToSolrWithParser(&sp.parser, virgoQuery); err != nil {
		return nil, err
	}

	// do some pre-analysis

	// just checking for single-term searches so this approach is sufficient:

	total := len(sp.parser.Titles) + len(sp.parser.Authors) + len(sp.parser.Subjects) + len(sp.parser.Keywords)

	sp.isTitleSearch = total == 1 && len(sp.parser.Titles) == 1
	sp.isKeywordSearch = total == 1 && len(sp.parser.Keywords) == 1

	return &sp, nil
}
