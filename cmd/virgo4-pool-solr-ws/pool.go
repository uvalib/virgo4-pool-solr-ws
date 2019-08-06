package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	//log "github.com/sirupsen/logrus"
)

// git commit used for this build; supplied at compile time
var gitCommit string

type poolVersion struct {
	BuildVersion string `json:"build,omitempty"`
	GoVersion    string `json:"go_version,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
}

type poolIdentity struct {
	Name string `json:"name,omitempty"`        // pool type
	Desc string `json:"description,omitempty"` // localized description
	URL  string `json:"public_url,omitempty"`  // public (service) url
}

type poolSolr struct {
	client               *http.Client
	url                  string
	availableFacets      map[string]solrRequestFacet
	virgoAvailableFacets []string
}

type poolContext struct {
	config       *poolConfig
	randomSource *rand.Rand
	identity     poolIdentity
	version      poolVersion
	solr         poolSolr
}

func buildVersion() string {
	files, _ := filepath.Glob("buildtag.*")
	if len(files) == 1 {
		return strings.Replace(files[0], "buildtag.", "", 1)
	}

	return "unknown"
}

func timeoutWithMinimum(str string, min int) int {
	val, err := strconv.Atoi(str)

	// fallback for invalid or nonsensical timeout values
	if err != nil || val < min {
		val = min
	}

	return val
}

func (p *poolContext) init(config *poolConfig) {
	p.config = config

	p.identity = poolIdentity{
		Name: config.poolType,
		Desc: config.poolDescription,
		URL:  config.poolServiceURL,
	}

	p.version = poolVersion{
		BuildVersion: buildVersion(),
		GoVersion:    fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		GitCommit:    gitCommit,
	}

	p.solr.url = fmt.Sprintf("%s/%s/%s", config.solrHost, config.solrCore, config.solrHandler)

	connTimeout := timeoutWithMinimum(config.solrConnTimeout, 5)
	readTimeout := timeoutWithMinimum(config.solrReadTimeout, 5)

	solrTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(connTimeout) * time.Second,
		}).Dial,
		TLSHandshakeTimeout: time.Duration(connTimeout) * time.Second,
	}

	p.solr.client = &http.Client{
		Timeout:   time.Duration(readTimeout) * time.Second,
		Transport: solrTransport,
	}

	type facetInfo struct {
		Facets []solrRequestFacet `json:"facets"`
	}

	var facets facetInfo

	if err := json.Unmarshal([]byte(config.solrAvailableFacets), &facets); err != nil {
		log.Printf("error parsing available facets json: %s", err.Error())
		os.Exit(1)
	}

	p.solr.availableFacets = make(map[string]solrRequestFacet)

	for _, facet := range facets.Facets {
		p.solr.virgoAvailableFacets = append(p.solr.virgoAvailableFacets, facet.Name)
		p.solr.availableFacets[facet.Name] = solrRequestFacet{Type: facet.Type, Field: facet.Field, Sort: facet.Sort, Limit: facet.Limit}
	}

	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))
}
