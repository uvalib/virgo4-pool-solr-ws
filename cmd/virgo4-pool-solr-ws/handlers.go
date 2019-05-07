package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func poolResultsHandler(c *gin.Context) {
	var req VirgoSearchRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("Invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	res, resErr := solrPoolResultsHandler(req)

	if resErr != nil {
		log.Printf("poolResultsHandler: error:  %s", resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func poolResultsRecordHandler(c *gin.Context) {
	var req VirgoSearchRequest

	req.Query.Id = c.Param("id")

	res, resErr := solrPoolResultsRecordHandler(req)

	if resErr != nil {
		log.Printf("poolResultsRecordHandler: error:  %s", resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func poolSummaryHandler(c *gin.Context) {
	var req VirgoSearchRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("Invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	res, resErr := solrPoolSummaryHandler(req)

	if resErr != nil {
		log.Printf("poolSummaryHandler: error:  %s", resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
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
