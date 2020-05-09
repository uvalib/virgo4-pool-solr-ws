package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/uvalib/virgo4-jwt/v4jwt"
)

func (p *poolContext) searchHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

	cl.logRequest()
	resp := s.handleSearchRequest()
	cl.logResponse(resp)

	if resp.err != nil {
		s.err("searchHandler: error: %s", resp.err.Error())
	}

	c.JSON(resp.status, resp.data)
}

func (p *poolContext) facetsHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

	cl.logRequest()
	resp := s.handleFacetsRequest()
	cl.logResponse(resp)

	if resp.err != nil {
		s.err("facetsHandler: error: %s", resp.err.Error())
	}

	c.JSON(resp.status, resp.data)
}

func (p *poolContext) resourceHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	// this is a single item, no grouping needed
	cl.opts.grouped = false

	s := searchContext{}
	s.init(p, &cl)

	// fill out Solr query directly, bypassing query syntax parser
	s.virgo.solrQuery = fmt.Sprintf(`id:"%s"`, c.Param("id"))

	// mark this as a resource request
	s.itemDetails = true

	cl.logRequest()
	resp := s.handleRecordRequest()
	cl.logResponse(resp)

	if resp.err != nil {
		s.err("resourceHandler: error: %s", resp.err.Error())
		c.String(resp.status, resp.err.Error())
		return
	}

	c.JSON(resp.status, resp.data)
}

func (p *poolContext) ignoreHandler(c *gin.Context) {
}

func (p *poolContext) versionHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	c.JSON(http.StatusOK, p.version)
}

func (p *poolContext) identifyHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	localizedIdentity := cl.localizedPoolIdentity(p)

	c.JSON(http.StatusOK, localizedIdentity)
}

func (p *poolContext) providersHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	localizedProviders := cl.localizedProviders(p)

	c.JSON(http.StatusOK, localizedProviders)
}

func (p *poolContext) healthCheckHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

	if s.client.opts.verbose == false {
		s.client.nolog = true
	}

	// fill out Solr query directly, bypassing query syntax parser
	s.virgo.solrQuery = "id:pingtest"

	cl.logRequest()
	ping := s.handlePingRequest()
	cl.logResponse(ping)

	// build response

	internalServiceError := false

	type hcResp struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message,omitempty"`
	}

	hcSolr := hcResp{Healthy: true}
	if ping.err != nil {
		internalServiceError = true
		hcSolr = hcResp{Healthy: false, Message: ping.err.Error()}
	}

	hcMap := make(map[string]hcResp)
	hcMap["solr"] = hcSolr

	hcStatus := http.StatusOK
	if internalServiceError == true {
		hcStatus = http.StatusInternalServerError
	}

	c.JSON(hcStatus, hcMap)
}

func getBearerToken(authorization string) (string, error) {
	components := strings.Split(strings.Join(strings.Fields(authorization), " "), " ")

	// must have two components, the first of which is "Bearer", and the second a non-empty token
	if len(components) != 2 || components[0] != "Bearer" || components[1] == "" {
		return "", fmt.Errorf("Invalid Authorization header: [%s]", authorization)
	}

	token := components[1]

	if token == "undefined" {
		return "", errors.New("bearer token is undefined")
	}

	return token, nil
}

func (p *poolContext) authenticateHandler(c *gin.Context) {
	token, err := getBearerToken(c.GetHeader("Authorization"))
	if err != nil {
		log.Printf("Authentication failed: [%s]", err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	claims, err := v4jwt.Validate(token, p.config.Global.Service.JWTKey)

	if err != nil {
		log.Printf("JWT signature for %s is invalid: %s", token, err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Set("claims", claims)
}
