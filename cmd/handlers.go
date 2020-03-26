package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uvalib/virgo4-jwt/v4jwt"
)

func (p *poolContext) searchHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

	resp := s.handleSearchRequest(c)

	if resp.err != nil {
		s.err("searchHandler: error: %s", resp.err.Error())
		c.String(resp.status, resp.err.Error())
		return
	}

	start := time.Now()
	c.JSON(resp.status, resp.data)
	cl.log("[CLIENT] response: %5d ms", int64(time.Since(start)/time.Millisecond))
}

func (p *poolContext) facetsHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	s := searchContext{}
	s.init(p, &cl)

	resp := s.handleFacetsRequest(c)

	if resp.err != nil {
		s.err("facetsHandler: error: %s", resp.err.Error())
		c.String(resp.status, resp.err.Error())
		return
	}

	start := time.Now()
	c.JSON(resp.status, resp.data)
	cl.log("[CLIENT] response: %5d ms", int64(time.Since(start)/time.Millisecond))
}

func (p *poolContext) resourceHandler(c *gin.Context) {
	cl := clientContext{}
	cl.init(p, c)

	// this is a single item, no grouping needed
	cl.opts.grouped = false

	s := searchContext{}
	s.init(p, &cl)

	// fill out Solr query directly, bypassing query syntax parser
	s.virgoReq.meta.solrQuery = fmt.Sprintf(`id:"%s"`, c.Param("id"))

	resp := s.handleRecordRequest()

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

	c.JSON(http.StatusOK, p.providers)
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
	s.virgoReq.meta.solrQuery = "id:pingtest"

	ping := s.handlePingRequest()

	// build response

	type hcResp struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message,omitempty"`
	}

	hcSolr := hcResp{Healthy: true}
	if ping.err != nil {
		hcSolr = hcResp{Healthy: false, Message: ping.err.Error()}
	}

	hcMap := make(map[string]hcResp)

	hcMap["solr"] = hcSolr

	c.JSON(ping.status, hcMap)
}

func (p *poolContext) getBearerToken(authorization string) (string, error) {
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
	token, err := p.getBearerToken(c.GetHeader("Authorization"))
	if err != nil {
		log.Printf("Authentication failed: [%s]", err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	claims, err := v4jwt.Validate(token, p.config.jwtKey)

	if err != nil {
		log.Printf("JWT signature for %s is invalid: %s", token, err.Error())
		log.Printf("continuing with no claims")
		//c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// save token and claims to context

	c.Set("jwt", token)
	c.Set("claims", claims)

	log.Printf("got bearer token: [%s]: %+v", token, claims)
}
