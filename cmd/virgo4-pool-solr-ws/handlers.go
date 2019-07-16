package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

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

	// fill out Solr query directly, bypassing query syntax parser
	s.virgoReq.solrQuery = fmt.Sprintf("id:%s", c.Param("id"))

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
	c.JSON(http.StatusOK, versions)
}

func identifyHandler(c *gin.Context) {
	iMap := make(map[string]string)

	iMap["name"] = pool.name
	iMap["description"] = pool.desc
	iMap["public_url"] = pool.url

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

	hcSolr := hcResp{}

	status := http.StatusOK
	hcSolr = hcResp{Healthy: true}

	if err := s.handlePingRequest(); err != nil {
		status = http.StatusInternalServerError
		hcSolr = hcResp{Healthy: false, Message: err.Error()}
	}

	hcMap["solr"] = hcSolr

	c.JSON(status, hcMap)
}

func getBearerToken(authorization string) (string, error) {
	components := strings.Split(strings.Join(strings.Fields(authorization), " "), " ")

	// must have two components, the first of which is "Bearer", and the second a non-empty token
	if len(components) != 2 || components[0] != "Bearer" || components[1] == "" {
		return "", fmt.Errorf("Invalid Authorization header: [%s]", authorization)
	}

	return components[1], nil
}

func authenticateHandler(c *gin.Context) {
	token, err := getBearerToken(c.Request.Header.Get("Authorization"))

	if err != nil {
		log.Printf("Authentication failed: [%s]", err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// do something with token

	log.Printf("got bearer token: [%s]", token)
}
