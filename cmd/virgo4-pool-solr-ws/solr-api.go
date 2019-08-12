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
	Debug        bool     `json:"debug,omitempty"`
	DefType      string   `json:"defType,omitempty"`
	Qt           string   `json:"qt,omitempty"`
	Start        int      `json:"start"`
	Rows         int      `json:"rows"`
	Fl           []string `json:"fl,omitempty"`
	Fq           []string `json:"fq,omitempty"`
	Q            string   `json:"q,omitempty"`
	GroupField   string   `json:"group.field"`
	GroupLimit   int      `json:"group.limit"`
	GroupNGroups bool     `json:"group.ngroups"`
	GroupMain    bool     `json:"group.main"`
	Group        bool     `json:"group"`
}

type solrRequestFacets map[string]solrRequestFacet

type solrRequestFacet struct {
	Name   string `json:"name,omitempty"` // used internally when initializing available facets
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

type solrMeta struct {
	client          *clientOptions
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
	reverseFacetMap map[string]string
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
	Score            float32  `json:"score,omitempty"`
	ID               string   `json:"id,omitempty"`
	Title            []string `json:"title_a,omitempty"`
	Subtitle         []string `json:"title_sub_a,omitempty"`
	Author           []string `json:"author_facet_a,omitempty"`
	Subject          []string `json:"subject_a,omitempty"`
	Language         []string `json:"language_a,omitempty"`
	Format           []string `json:"format_a,omitempty"`
	Library          []string `json:"library_a,omitempty"`
	CallNumber       []string `json:"call_number_a,omitempty"`
	CallNumberBroad  []string `json:"call_number_broad_a,omitempty"`
	CallNumberNarrow []string `json:"call_number_narrow_a,omitempty"`
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

type solrResponseGroup struct {
	GroupValue string                `json:"groupValue,omitempty"`
	DocList    solrResponseDocuments `json:"doclist,omitempty"`
}

type solrResponseGrouping struct {
	Matches int                 `json:"matches,omitempty"`
	NGroups int                 `json:"ngroups,omitempty"`
	Groups  []solrResponseGroup `json:"groups,omitempty"`
}

type solrResponseGrouped struct {
	WorkTitle2KeySort solrResponseGrouping `json:"work_title2_key_sort,omitempty"`
}

type solrError struct {
	Metadata []string `json:"metadata,omitempty"`
	Msg      string   `json:"msg,omitempty"`
	Code     int      `json:"code,omitempty"`
}

type solrResponse struct {
	ResponseHeader solrResponseHeader     `json:"responseHeader,omitempty"`
	Response       solrResponseDocuments  `json:"response,omitempty"` // ungrouped records
	Grouped        solrResponseGrouped    `json:"grouped,omitempty"`  // grouped records
	FacetsRaw      map[string]interface{} `json:"facets,omitempty"`
	Facets         solrResponseFacets     // will be parsed from FacetsRaw
	Error          solrError              `json:"error,omitempty"`
	meta           *solrMeta              // pointer to struct in corresponding solrRequest
}
