package main

// http query parameters

/*
for reference:

type solrQueryParams struct {
	q                      string   // query
	fq                     []string // filter quer{y,ies}
	sort                   string   // sort field or function with asc|desc
	start                  string   // number of leading documents to skip
	rows                   string   // number of documents to return after 'start'
	fl                     string   // field list, comma separated
	df                     string   // default search field
	wt                     string   // writer type (response format)
	defType                string   // query parser (lucene, dismax, ...)
	debugQuery             string   // timing & results ("on" or omit)
	debug                  string
	explainOther           string
	timeAllowed            string
	segmentTerminatedEarly string
	omitHeader             string
}
*/

type solrParamsMap map[string]string

type solrRequest struct {
	params solrParamsMap
}

// json response

/*
https://doc.lucidworks.com/fusion/3.1/REST_API_Reference/Solr-API.html
{
    "responseHeader": {
        "status": 0,
        "QTime": 2,
        "params": {
            "fl": "title",
            "q": "solr",
            "wt": "json",
            "rows": "2"
        }
    },
    "response": {
        "numFound": 52,
        "start": 0,
        "maxScore": 1.23,
        "docs": [
            {
                "title": [
                    "Solr and SolrAdmin APIs - Fusion Documentation - Lucidworks"
                ],
				"score": 1.23
            },
            {
                "title": [
                    "Search Clusters - Fusion Documentation - Lucidworks"
                ],
				"score": 0.98
            }
        ]
    }
}
*/

type solrResponseHeader struct {
	Status int           `json:"status,omitempty"`
	QTime  int           `json:"QTime,omitempty"`
	Params solrParamsMap `json:"params,omitempty"`
}

//type solrDocument map[string]interface{}
type solrDocument struct {
	Score    float32  `json:"score,omitempty"`
	Id       string   `json:"id,omitempty"`
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
}

/*
type solrResponse struct {
	json map[string]interface{}
}
*/
