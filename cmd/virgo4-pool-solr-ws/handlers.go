package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (p *poolContext) searchHandler(c *gin.Context) {
	cl := clientOptions{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

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

func (p *poolContext) resourceHandler(c *gin.Context) {
	cl := clientOptions{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

	// fill out Solr query directly, bypassing query syntax parser
	s.virgoReq.meta.solrQuery = fmt.Sprintf("id:%s", c.Param("id"))

	virgoRes, err := s.handleRecordRequest()

	if err != nil {
		s.err("resourceHandler: error: %s", err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, virgoRes)
}

func (p *poolContext) ignoreHandler(c *gin.Context) {
}

func (p *poolContext) versionHandler(c *gin.Context) {
	cl := clientOptions{}
	cl.init(p, c)

	c.JSON(http.StatusOK, p.version)
}

func (p *poolContext) identifyHandler(c *gin.Context) {
	cl := clientOptions{}
	cl.init(p, c)

	localizedIdentity := cl.localizedPoolIdentity(p)

	c.JSON(http.StatusOK, localizedIdentity)
}

func (p *poolContext) healthCheckHandler(c *gin.Context) {
	cl := clientOptions{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

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

func (p *poolContext) getBearerToken(authorization string) (string, error) {
	components := strings.Split(strings.Join(strings.Fields(authorization), " "), " ")

	// must have two components, the first of which is "Bearer", and the second a non-empty token
	if len(components) != 2 || components[0] != "Bearer" || components[1] == "" {
		return "", fmt.Errorf("Invalid Authorization header: [%s]", authorization)
	}

	return components[1], nil
}

func (p *poolContext) authenticateHandler(c *gin.Context) {
	token, err := p.getBearerToken(c.GetHeader("Authorization"))

	if err != nil {
		log.Printf("Authentication failed: [%s]", err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// do something with token

	log.Printf("got bearer token: [%s]", token)
}
