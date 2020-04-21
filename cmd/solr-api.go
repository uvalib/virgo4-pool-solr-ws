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
	titles                []string
	keywords              []string
}

type solrRequestParams struct {
	DefType string   `json:"defType,omitempty"`
	Qt      string   `json:"qt,omitempty"`
	Sort    string   `json:"sort,omitempty"`
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
	config *poolConfigFacet
}

type solrRequestJSON struct {
	Params solrRequestParams `json:"params"`
	Facets solrRequestFacets `json:"facet,omitempty"`
}

type solrMeta struct {
	client       *clientContext
	parserInfo   *solrParserInfo
	warnings     []string
	maxScore     float32
	firstDoc     *solrDocument
	start        int
	numGroups    int                        // for grouped records
	totalGroups  int                        // for grouped records
	numRecords   int                        // for grouped or ungrouped records
	totalRecords int                        // for grouped or ungrouped records
	numRows      int                        // for client pagination -- numGroups or numRecords
	totalRows    int                        // for client pagination -- totalGroups or totalRecords
	selectionMap map[string]map[string]bool // to track what filters have been applied by the client
	sort         VirgoSort                  // to hold requested (or defaulted) sort info
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
	AlternateID          []string `json:"alternate_id_a,omitempty"`
	AnonAvailability     []string `json:"anon_availability_a,omitempty"`
	Author               []string `json:"author_facet_a,omitempty"`
	Barcode              []string `json:"barcode_a,omitempty"`
	CallNumber           []string `json:"call_number_a,omitempty"`
	CallNumberBroad      []string `json:"call_number_broad_a,omitempty"`
	CallNumberNarrow     []string `json:"call_number_narrow_a,omitempty"`
	Collection           []string `json:"collection_a,omitempty"`
	DataSource           []string `json:"data_source_a,omitempty"`
	Director             []string `json:"author_director_a,omitempty"`
	Format               []string `json:"format_a,omitempty"`
	FullRecord           string   `json:"fullrecord,omitempty"`
	ID                   string   `json:"id,omitempty"`
	ISBN                 []string `json:"isbn_a,omitempty"`
	ISSN                 []string `json:"issn_a,omitempty"`
	Identifier           []string `json:"identifier_a,omitempty"`
	IndividualCallNumber []string `json:"individual_call_number_a,omitempty"`
	LCCN                 []string `json:"lccn_a,omitempty"`
	Language             []string `json:"language_a,omitempty"`
	Library              []string `json:"library_a,omitempty"`
	Location             []string `json:"location2_a,omitempty"`
	Note                 []string `json:"note_a,omitempty"`
	OCLC                 []string `json:"oclc_a,omitempty"`
	PDFURL               []string `json:"pdf_url_a,omitempty"`
	Pool                 []string `json:"pool_a,omitempty"`
	PublicationDate      string   `json:"published_date,omitempty"`
	PublicationDateRange []string `json:"published_daterange,omitempty"`
	Published            []string `json:"published_a,omitempty"`
	Region               []string `json:"region_a,omitempty"`
	ReleaseDate          []string `json:"release_a,omitempty"`
	RightsWrapperURL     []string `json:"rights_wrapper_url_a,omitempty"`
	Score                float32  `json:"score,omitempty"`
	Series               []string `json:"title_series_a,omitempty"`
	Subject              []string `json:"subject_a,omitempty"`
	SubjectSummary       []string `json:"subject_summary_a,omitempty"`
	Subtitle             []string `json:"title_sub_a,omitempty"`
	ThumbnailURL         []string `json:"thumbnail_url_a,omitempty"`
	Title                []string `json:"title_a,omitempty"`
	UPC                  []string `json:"upc_a,omitempty"`
	URL                  []string `json:"url_a,omitempty"`
	URLIIIFImage         string   `json:"url_iiif_image_stored,omitempty"`
	URLIIIFManifest      string   `json:"url_iiif_manifest_stored,omitempty"`
	URLLabel             []string `json:"url_label_a,omitempty"`
	UVAAvailability      []string `json:"uva_availability_a,omitempty"`
	VideoGenre           []string `json:"video_genre_a,omitempty"`
	WorkIdentifier       []string `json:"workIdentifier_a,omitempty"`
	WorkLocation         []string `json:"workLocation_a,omitempty"`
	WorkPhysicalDetails  []string `json:"workPhysicalDetails_a,omitempty"`
	WorkTitle2KeySort    string   `json:"work_title2_key_sort,omitempty"`
	WorkTitle3KeySort    string   `json:"work_title3_key_sort,omitempty"`
	WorkType             []string `json:"workType_a,omitempty"`
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
