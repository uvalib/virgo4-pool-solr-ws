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

	return &sp, nil
}
