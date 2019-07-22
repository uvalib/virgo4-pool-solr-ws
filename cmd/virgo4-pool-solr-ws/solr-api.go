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

/*
Query parameters 	JSON field equivalent
q					query
fq					filter
offset				start
limit				rows
sort				sort
json.facet			facet
json.<param_name>	<param_name>

Unmapped parameters (or original query parameters above) can be passed in "params" block
*/

type solrRequestParams struct {
	Debug   bool     `json:"debug,omitempty"`
	DefType string   `json:"defType,omitempty"`
	Qt      string   `json:"qt,omitempty"`
	Start   int      `json:"start"`
	Rows    int      `json:"rows"`
	Fl      []string `json:"fl,omitempty"`
	Fq      []string `json:"fq,omitempty"`
	Q       string   `json:"q,omitempty"`
}

type solrRequestFacets map[string]solrRequestFacet

type solrRequestFacet struct {
	Type   string `json:"type"`
	Field  string `json:"field"`
	Sort   string `json:"sort,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type solrRequestJSON struct {
	Params solrRequestParams `json:"params"`
	Facets solrRequestFacets `json:"facet,omitempty"`
}

type solrRequestMeta struct {
	client          *clientOptions
	parserInfo      *solrParserInfo
	warnings        []string
	advertiseFacets bool
}

type solrRequest struct {
	json solrRequestJSON
	meta solrRequestMeta
}

type solrResponseHeader struct {
	Status int `json:"status,omitempty"`
	QTime  int `json:"QTime,omitempty"`
}

type solrDocument struct {
	Score    float32  `json:"score,omitempty"`
	ID       string   `json:"id,omitempty"`
	Title    []string `json:"title_a,omitempty"`
	Subtitle []string `json:"title_sub_a,omitempty"`
	Author   []string `json:"author_a,omitempty"`
	// etc.
}

type solrBucket struct {
	Val   string `json:"val"`
	Count int    `json:"count"`
}

type solrResponseFacet struct {
	Buckets []solrBucket `json:"buckets,omitempty"`
}

type solrResponseFacets map[string]solrResponseFacet

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
	ResponseHeader solrResponseHeader     `json:"responseHeader,omitempty"`
	Response       solrResponseBody       `json:"response,omitempty"`
	FacetsRaw      map[string]interface{} `json:"facets,omitempty"`
	Facets         solrResponseFacets     // will be parsed from FacetsRaw
	Error          solrError              `json:"error,omitempty"`
	solrReq        *solrRequest           // used internally to reference fields in original request
}
