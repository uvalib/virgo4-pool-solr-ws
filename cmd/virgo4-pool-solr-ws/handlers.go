package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
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
	vMap := make(map[string]string)

	vMap["build"] = Version()

	c.JSON(http.StatusOK, vMap)
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

	hcRes := hcResp{}

	if err := s.handlePingRequest(); err != nil {
		hcRes = hcResp{Healthy: false, Message: err.Error()}
	} else {
		hcRes = hcResp{Healthy: true}
	}

	hcMap["solr"] = hcRes

	c.JSON(http.StatusOK, hcMap)
}

func getBearerToken(authorization string) (string, error) {
	// shortcut to avoid unnecessary regex
	if authorization == "" {
		return "", errors.New("Missing/empty Authorization header")
	}

	// clean up extraneous spaces in header value before splitting
	ends := regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	middle := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	cleaned := middle.ReplaceAllString(ends.ReplaceAllString(authorization, ""), " ")

	pieces := strings.Split(cleaned, " ")

	// must have two components, the first of which is "Bearer", and the second a non-empty token
	if len(pieces) != 2 || pieces[0] != "Bearer" || pieces[1] == "" {
		return "", fmt.Errorf("Invalid Authorization header: [%s]", authorization)
	}

	return pieces[1], nil
}

func authenticateHandler(c *gin.Context) {
	token, err := getBearerToken(c.Request.Header.Get("Authorization"))

	if err != nil {
		log.Printf("authentication failed: [%s]", err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// do something with token

	log.Printf("got bearer token: [%s]", token)
}
