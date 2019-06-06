package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

func virgoQueryValidate(virgoQuery string) (bool, string) {
	return v4parser.Validate(virgoQuery)
}

func virgoQueryConvertToSolr(virgoQuery string) (string, error) {
	return v4parser.ConvertToSolr(virgoQuery)
}
