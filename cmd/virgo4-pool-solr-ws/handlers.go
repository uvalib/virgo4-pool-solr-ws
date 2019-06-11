package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func searchHandler(c *gin.Context) {
	s := newSearchContext(c)

	if err := c.BindJSON(&s.virgoReq); err != nil {
		s.err("searchHandler: invalid request: %s", err.Error())
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	s.log("query: [%s]", s.virgoReq.Query)

	virgoRes, err := s.handleSearchRequest()

	if err != nil {
		s.err("searchHandler: error: %s", err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, virgoRes)
}

func resourceHandler(c *gin.Context) {
	s := newSearchContext(c)

	s.virgoReq.Query = fmt.Sprintf("id:%s", c.Param("id"))

	virgoRes, err := s.handleRecordRequest()

	if err != nil {
		s.err("resourceHandler: error: %s", err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, virgoRes)
}

func ignoreHandler(c *gin.Context) {
}

func versionHandler(c *gin.Context) {
	vMap := make(map[string]string)

	vMap["build"] = Version()

	c.JSON(http.StatusOK, vMap)
}

func identifyHandler(c *gin.Context) {
	iMap := make(map[string]string)

	iMap["name"] = pool.name
	iMap["description"] = pool.desc

	c.JSON(http.StatusOK, iMap)
}

func healthCheckHandler(c *gin.Context) {
	s := newSearchContext(c)

	s.client.nolog = true

	s.virgoReq.Query = "id:pingtest"

	type hcResp struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message,omitempty"`
	}

	hcMap := make(map[string]hcResp)

	hcRes := hcResp{}

	if err := s.handlePingRequest(); err != nil {
		hcRes = hcResp{Healthy: false, Message: err.Error()}
	} else {
		hcRes = hcResp{Healthy: true}
	}

	hcMap["solr"] = hcRes

	c.JSON(http.StatusOK, hcMap)
}
