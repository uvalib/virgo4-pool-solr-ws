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
	randomSource *rand.Rand
	config       *poolConfig
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

func (p *poolContext) initIdentity() {
	p.identity = poolIdentity{
		Name: p.config.poolType,
		Desc: p.config.poolDescription,
		URL:  p.config.poolServiceURL,
	}
}

func (p *poolContext) initVersion() {
	p.version = poolVersion{
		BuildVersion: buildVersion(),
		GoVersion:    fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		GitCommit:    gitCommit,
	}
}

func (p *poolContext) initSolr() {
	// client setup

	connTimeout := timeoutWithMinimum(p.config.solrConnTimeout, 5)
	readTimeout := timeoutWithMinimum(p.config.solrReadTimeout, 5)

	solrTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(connTimeout) * time.Second,
		}).Dial,
		TLSHandshakeTimeout: time.Duration(connTimeout) * time.Second,
	}

	solrClient := &http.Client{
		Timeout:   time.Duration(readTimeout) * time.Second,
		Transport: solrTransport,
	}

	// facet setup

	type facetInfo struct {
		Facets []solrRequestFacet `json:"facets"`
	}

	var facets facetInfo

	if err := json.Unmarshal([]byte(p.config.solrAvailableFacets), &facets); err != nil {
		log.Printf("error parsing available facets json: %s", err.Error())
		os.Exit(1)
	}

	availableFacets := make(map[string]solrRequestFacet)
	var virgoAvailableFacets []string

	for _, facet := range facets.Facets {
		virgoAvailableFacets = append(virgoAvailableFacets, facet.Name)
		availableFacets[facet.Name] = solrRequestFacet{Type: facet.Type, Field: facet.Field, Sort: facet.Sort, Limit: facet.Limit}
	}

	p.solr = poolSolr{
		url: fmt.Sprintf("%s/%s/%s", p.config.solrHost, p.config.solrCore, p.config.solrHandler),
		client: solrClient,
		availableFacets: availableFacets,
		virgoAvailableFacets: virgoAvailableFacets,
	}
}

func (p *poolContext) init(config *poolConfig) {
	p.config = config

	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	p.initIdentity()
	p.initVersion()
	p.initSolr()
}
