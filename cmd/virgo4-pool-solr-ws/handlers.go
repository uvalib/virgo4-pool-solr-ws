package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func searchHandler(c *gin.Context) {
	var req VirgoSearchRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("searchHandler: invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	res, resErr := solrSearchHandler(req)

	if resErr != nil {
		log.Printf("searchHandler: error: %s", resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func resourceHandler(c *gin.Context) {
	id := c.Param("id")

	res, resErr := solrRecordHandler(id)

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
	var desc string

	// pool description will vary based on pool type, and will eventually
	// be localized as well based on browser language settings

	switch config.poolType.value {
	case "catalog":
		desc = "The UVA Library Catalog"

	default:
		desc = fmt.Sprintf("The UVA Library Catalog (%s)", strings.Title(config.poolType.value))
	}

	iMap := make(map[string]string)

	iMap["name"] = config.poolType.value
	iMap["description"] = desc

	c.JSON(http.StatusOK, iMap)
}

func healthCheckHandler(c *gin.Context) {
	hcMap := make(map[string]string)

	// FIXME

	hcMap[program] = "true"
	hcMap["Solr"] = "true"

	c.JSON(http.StatusOK, hcMap)
}
