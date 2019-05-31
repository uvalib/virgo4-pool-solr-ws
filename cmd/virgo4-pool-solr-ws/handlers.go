package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// options set by client
type FormatOptions struct {
	debug bool
}

func parseBoolOption(opt string) bool {
	val := false

	if b, err := strconv.ParseBool(opt); err == nil && b == true {
		val = true
	}

	return val
}

func getFormatOptions(c *gin.Context) FormatOptions {
	var options FormatOptions

	options.debug = parseBoolOption(c.Query("debug"))

	return options
}

func searchHandler(c *gin.Context) {
	var req VirgoSearchRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("searchHandler: invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	opts := getFormatOptions(c)

	res, resErr := solrSearchHandler(req, opts)

	if resErr != nil {
		log.Printf("searchHandler: error: %s", resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func resourceHandler(c *gin.Context) {
	id := c.Param("id")

	opts := getFormatOptions(c)

	res, resErr := solrRecordHandler(id, opts)

	if resErr != nil {
		log.Printf("resourceHandler: error: %s", resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func ignoreHandler(c *gin.Context) {
}

func versionHandler(c *gin.Context) {

	vMap := make(map[string]string)

	vMap["version"] = Version()

	c.JSON(http.StatusOK, vMap)
}

func identifyHandler(c *gin.Context) {
	iMap := make(map[string]string)

	iMap["name"] = pool.name
	iMap["description"] = pool.desc

	c.JSON(http.StatusOK, iMap)
}

func healthCheckHandler(c *gin.Context) {
	hcMap := make(map[string]string)

	// FIXME

	hcMap["self"] = "true"
	hcMap["Solr"] = "true"

	c.JSON(http.StatusOK, hcMap)
}
