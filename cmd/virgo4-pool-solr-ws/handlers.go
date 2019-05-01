package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
type solrQueryParameters struct {
	q string `json:"q,omitempty" binding:"required"` // query
	fq []string `json:"fq,omitempty` // filter quer{y,ies}
	sort string `json:"sort,omitempty` // sort field or function with asc|desc
	start string `json:"start,omitempty` // number of leading documents to skip
	rows string `json:"rows,omitempty` // number of documents to return after 'start'
	fl string `json:"fl,omitempty` // field list, comma separated
	df string `json:"df,omitempty` // default search field
	wt string `json:"wt,omitempty` // writer type (response format)
	defType string `json:"defType,omitempty` // query parser (lucene, dismax, ...)
	debugQuery string `json:"debugQuery,omitempty` // timing & results ("on" or omit)
	debug string `json:"debug,omitempty`
	explainOther string `json:"explainOther,omitempty`
	timeAllowed string `json:"timeAllowed,omitempty`
	segmentTerminatedEarly string `json:"segmentTerminatedEarly,omitempty`
	omitHeader string `json:"omitHeader,omitempty`
	debug string `json:"debug,omitempty`
}
*/

//type solrRequest map[string]interface{} // converted to Solr query parameters
//type solrResponse map[string]interface{} // as-is from Solr

func poolResultsHandler(c *gin.Context) {
	var req VirgoPoolResultsRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("Invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	solrReq, solrReqErr := solrPoolResultsRequest(req)

	if solrReqErr != nil {
		log.Printf("query build error: %s", solrReqErr.Error())
		c.String(http.StatusInternalServerError, solrReqErr.Error())
		return
	}

	solrRes, solrResErr := solrQuery(solrReq)

	if solrResErr != nil {
		log.Printf("query execute error: %s", solrResErr.Error())
		c.String(http.StatusInternalServerError, solrResErr.Error())
		return
	}

	c.JSON(http.StatusOK, solrRes.json)
}

func poolResultsRecordHandler(c *gin.Context) {
	var req VirgoPoolResultsRecordRequest

	req.Id = c.Param("id")

	solrReq, solrReqErr := solrPoolResultsRecordRequest(req)

	if solrReqErr != nil {
		log.Printf("query build error: %s", solrReqErr.Error())
		c.String(http.StatusInternalServerError, solrReqErr.Error())
		return
	}

	solrRes, solrResErr := solrQuery(solrReq)

	if solrResErr != nil {
		log.Printf("query execute error: %s", solrResErr.Error())
		c.String(http.StatusInternalServerError, solrResErr.Error())
		return
	}

	c.JSON(http.StatusOK, solrRes.json)
}

func poolSummaryHandler(c *gin.Context) {
	var req VirgoPoolSummaryRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("Invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	solrReq, solrReqErr := solrPoolSummaryRequest(req)

	if solrReqErr != nil {
		log.Printf("query build error: %s", solrReqErr.Error())
		c.String(http.StatusInternalServerError, solrReqErr.Error())
		return
	}

	solrRes, solrResErr := solrQuery(solrReq)

	if solrResErr != nil {
		log.Printf("query execute error: %s", solrResErr.Error())
		c.String(http.StatusInternalServerError, solrResErr.Error())
		return
	}

	c.JSON(http.StatusOK, solrRes.json)
}

func ignoreHandler(c *gin.Context) {
}

func versionHandler(c *gin.Context) {
	c.String(http.StatusOK, "%s version %s", program, version)
}

func healthCheckHandler(c *gin.Context) {
	hcMap := make(map[string]string)

	hcMap[program] = "true"
	hcMap["Solr"] = "true"

	c.JSON(http.StatusOK, hcMap)
}

func metricsHandler(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}
