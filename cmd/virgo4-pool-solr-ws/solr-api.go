package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

type solrParserInfo struct {
	query  string
	parser v4parser.SolrParser
	// convenience flags based on parser results
	isSingleTitleSearch   bool
	isSingleKeywordSearch bool
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
	Type          string `json:"type"`
	Field         string `json:"field"`
	Sort          string `json:"sort,omitempty"`
	Offset        int    `json:"offset,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	exposedValues []string
}

type solrRequestJSON struct {
	Params solrRequestParams `json:"params"`
	Facets solrRequestFacets `json:"facet,omitempty"`
}

type solrMeta struct {
	client          *clientContext
	parserInfo      *solrParserInfo
	warnings        []string
	advertiseFacets bool
	maxScore        float32
	firstDoc        *solrDocument
	start           int
	numGroups       int // for grouped records
	totalGroups     int // for grouped records
	numRecords      int // for grouped or ungrouped records
	totalRecords    int // for grouped or ungrouped records
	numRows         int // for client pagination -- numGroups or numRecords
	totalRows       int // for client pagination -- totalGroups or totalRecords
}

type solrRequest struct {
	json solrRequestJSON
	meta solrMeta
}

type solrResponseHeader struct {
	Status int `json:"status,omitempty"`
	QTime  int `json:"QTime,omitempty"`
}

type solrDocument struct {
	// for performance reasons, pools should be configured to only request the fields below
	Score             float32  `json:"score,omitempty"`
	ID                string   `json:"id,omitempty"`
	Pool              []string `json:"pool_f,omitempty"`
	WorkTitle2KeySort string   `json:"work_title2_key_sort,omitempty"`
	Title             []string `json:"title_a,omitempty"`
	Subtitle          []string `json:"title_sub_a,omitempty"`
	Author            []string `json:"author_facet_a,omitempty"`
	Subject           []string `json:"subject_a,omitempty"`
	Language          []string `json:"language_a,omitempty"`
	Format            []string `json:"format_a,omitempty"`
	Library           []string `json:"library_a,omitempty"`
	Location          []string `json:"location2_a,omitempty"`
	CallNumber        []string `json:"call_number_a,omitempty"`
	CallNumberBroad   []string `json:"call_number_broad_a,omitempty"`
	CallNumberNarrow  []string `json:"call_number_narrow_a,omitempty"`
	AnonAvailability  []string `json:"anon_availability_a,omitempty"`
	UVAAvailability   []string `json:"uva_availability_a,omitempty"`
	ISBN              []string `json:"isbn_a,omitempty"`
	ISSN              []string `json:"issn_a,omitempty"`
	OCLC              []string `json:"oclc_a,omitempty"`
	LCCN              []string `json:"lccn_a,omitempty"`
	UPC               []string `json:"upc_a,omitempty"`
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

type solrResponseDocuments struct {
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
	Response       solrResponseDocuments  `json:"response,omitempty"`
	FacetsRaw      map[string]interface{} `json:"facets,omitempty"`
	Facets         solrResponseFacets     // will be parsed from FacetsRaw
	Error          solrError              `json:"error,omitempty"`
	meta           *solrMeta              // pointer to struct in corresponding solrRequest
}
