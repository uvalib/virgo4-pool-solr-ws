package main

import (
	"github.com/uvalib/virgo4-parser/v4parser"
)

type solrParserInfo struct {
	query  string
	parser v4parser.SolrParser
	// convenience flags based on parser results
	isSingleTitleSearch      bool
	isSingleKeywordSearch    bool
	isSingleIdentifierSearch bool
	isFulltextSearch         bool
	titles                   []string
	keywords                 []string
	fulltexts                []string
	identifiers              []string
}

type solrRequestParams struct {
	DefType    string   `json:"defType,omitempty"`
	Qt         string   `json:"qt,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	Start      int      `json:"start"`
	Rows       int      `json:"rows"`
	Fl         []string `json:"fl,omitempty"`
	Fq         []string `json:"fq,omitempty"`
	Q          string   `json:"q,omitempty"`
	DebugQuery string   `json:"debugQuery,omitempty"`

	// highlighter options
	Hl                  string   `json:"hl,omitempty"`
	HlMethod            string   `json:"hl.method,omitempty"`
	HlFl                []string `json:"hl.fl,omitempty"`
	HlSnippets          string   `json:"hl.snippets,omitempty"`
	HlFragsize          string   `json:"hl.fragsize,omitempty"`
	HlFragsizeIsMinimum string   `json:"hl.fragsizeIsMinimum,omitempty"`
	HlFragAlignRatio    string   `json:"hl.fragAlignRatio,omitempty"`
	HlMaxAnalyzedChars  string   `json:"hl.maxAnalyzedChars,omitempty"`
	HlMultiTermQuery    string   `json:"hl.multiTermQuery,omitempty"`
	HlTagPre            string   `json:"hl.tag.pre,omitempty"`
	HlTagPost           string   `json:"hl.tag.post,omitempty"`
}

type solrRequestSubFacet struct {
	GroupCount string `json:"group_count"`
}

type solrRequestFacet struct {
	Type     string              `json:"type,omitempty"`
	Field    string              `json:"field,omitempty"`
	Query    string              `json:"query,omitempty"`
	Sort     string              `json:"sort,omitempty"`
	Offset   int                 `json:"offset,omitempty"`
	Limit    int                 `json:"limit,omitempty"`
	MinCount int                 `json:"mincount,omitempty"`
	Facet    solrRequestSubFacet `json:"facet,omitempty"`
	config   *poolConfigFilter
}

type solrRequestJSON struct {
	Params solrRequestParams            `json:"params"`
	Facets map[string]*solrRequestFacet `json:"facet,omitempty"`
}

type solrMeta struct {
	client         *clientContext
	parserInfo     *solrParserInfo
	warnings       []string
	maxScore       float32
	firstDoc       *solrDocument
	start          int
	numGroups      int                          // for grouped records
	totalGroups    int                          // for grouped records
	numRecords     int                          // for grouped or ungrouped records
	totalRecords   int                          // for grouped or ungrouped records
	numRows        int                          // for client pagination -- numGroups or numRecords
	totalRows      int                          // for client pagination -- totalGroups or totalRecords
	selectionMap   map[string]map[string]string // to track what filters have been applied by the client
	internalFacets map[string]*solrRequestFacet // to track internal facet info for externally-advertised facets
	requestFacets  map[string]*solrRequestFacet // to track facets sent in the solr request
}

type solrRequest struct {
	json solrRequestJSON
	meta solrMeta
}

type solrResponseHeader struct {
	Status int `json:"status,omitempty"`
	QTime  int `json:"QTime,omitempty"`
}

type solrDocument map[string]interface{}

type solrBucket struct {
	Val        string `json:"val"`
	Count      int    `json:"count"`
	GroupCount int    `json:"group_count"`
}

type solrResponseFacet struct {
	Count      int          `json:"count"`
	GroupCount int          `json:"group_count"`
	Buckets    []solrBucket `json:"buckets,omitempty"`
}

type solrResponseDocuments struct {
	NumFound int            `json:"numFound,omitempty"`
	Start    int            `json:"start,omitempty"`
	MaxScore float32        `json:"maxScore,omitempty"`
	Docs     []solrDocument `json:"docs,omitempty"`
}

type solrResponseHighlighting map[string]map[string][]string

type solrError struct {
	Metadata []string `json:"metadata,omitempty"`
	Msg      string   `json:"msg,omitempty"`
	Code     int      `json:"code,omitempty"`
}

// a catch-all for search and ping responses
type solrResponse struct {
	ResponseHeader solrResponseHeader           `json:"responseHeader,omitempty"`
	Response       solrResponseDocuments        `json:"response,omitempty"`
	Highlighting   solrResponseHighlighting     `json:"highlighting,omitempty"`
	Debug          interface{}                  `json:"debug,omitempty"`
	FacetsRaw      map[string]interface{}       `json:"facets,omitempty"`
	Facets         map[string]solrResponseFacet // will be parsed from FacetsRaw
	Terms          map[string][]interface{}     `json:"terms,omitempty"`
	Error          solrError                    `json:"error,omitempty"`
	Status         string                       `json:"status,omitempty"`
	meta           *solrMeta                    // pointer to struct in corresponding solrRequest
}
