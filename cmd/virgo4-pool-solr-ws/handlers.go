package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func searchHandler(c *gin.Context) {
	client := getClientOptions(c)

	var req VirgoSearchRequest

	if err := c.BindJSON(&req); err != nil {
		log.Printf("[%s] searchHandler: invalid request: %s", client.reqId, err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	log.Printf("[%s] query: [%s]", client.reqId, req.Query)

	res, resErr := solrSearchHandler(req, client)

	if resErr != nil {
		log.Printf("[%s] searchHandler: error: %s", client.reqId, resErr.Error())
		c.String(http.StatusInternalServerError, resErr.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func resourceHandler(c *gin.Context) {
	id := c.Param("id")

	client := getClientOptions(c)

	res, resErr := solrRecordHandler(id, client)

	if resErr != nil {
		log.Printf("[%s] resourceHandler: error: %s", client.reqId, resErr.Error())
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
