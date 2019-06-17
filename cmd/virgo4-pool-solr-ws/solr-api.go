package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

type solrParserInfo struct {
	query  string
	parser v4parser.SolrParser
	// convenience flags based on parser results
	isTitleSearch   bool
	isKeywordSearch bool
}

type solrParamsMap map[string]string

type solrRequest struct {
	parserInfo *solrParserInfo
	params     solrParamsMap
}

type solrResponseHeader struct {
	Status int           `json:"status,omitempty"`
	QTime  int           `json:"QTime,omitempty"`
	Params solrParamsMap `json:"params,omitempty"`
}

type solrDocument struct {
	Score    float32  `json:"score,omitempty"`
	ID       string   `json:"id,omitempty"`
	Title    []string `json:"title_a,omitempty"`
	Subtitle []string `json:"title_sub_a,omitempty"`
	Author   []string `json:"author_a,omitempty"`
	// etc.
}

type solrResponseBody struct {
	NumFound int            `json:"numFound,omitempty"`
	Start    int            `json:"start,omitempty"`
	MaxScore float32        `json:"maxScore,omitempty"`
	Docs     []solrDocument `json:"docs,omitempty"`
}

type solrError struct {
	Metadata []string `json:"metadata,omitempty"`
	Msg      string   `json:"msg,omitempty"`
	Code     int      `json:"code,omitempty"`
}

type solrResponse struct {
	ResponseHeader solrResponseHeader `json:"responseHeader,omitempty"`
	Response       solrResponseBody   `json:"response,omitempty"`
	Error          solrError          `json:"error,omitempty"`
	parserInfo     *solrParserInfo    // used internally; pointer to one in solrRequest
}
