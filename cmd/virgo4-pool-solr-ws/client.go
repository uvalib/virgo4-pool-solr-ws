package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	//log "github.com/sirupsen/logrus"
)

// options set by or per client
type clientOptions struct {
	start   time.Time // internally set
	reqID   string    // internally generated
	nolog   bool      // internally set
	debug   bool      // client requested -- controls whether debug info is added to pool results
	intuit  bool      // client requested -- controls whether intuited (speculative) searches are performed
	verbose bool      // client requested -- controls whether verbose Solr requests/responses are logged
	grouped bool      // client requested -- controls whether Solr results are grouped
}

func boolOptionWithFallback(opt string, fallback bool) bool {
	var err error
	var val bool

	if val, err = strconv.ParseBool(opt); err != nil {
		val = fallback
	}

	return val
}

func (client *clientOptions) init(c *gin.Context) {
	client.start = time.Now()
	client.reqID = fmt.Sprintf("%0x", pool.randomSource.Uint64())
	client.debug = boolOptionWithFallback(c.Query("debug"), false)
	client.intuit = boolOptionWithFallback(c.Query("intuit"), true)
	client.verbose = boolOptionWithFallback(c.Query("verbose"), false)
	client.grouped = boolOptionWithFallback(c.Query("grouped"), false)

	query := ""
	if c.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", c.Request.URL.RawQuery)
	}

	client.log("%s %s%s", c.Request.Method, c.Request.URL.Path, query)
}

func (client *clientOptions) printf(prefix, format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)

	if prefix != "" {
		str = strings.Join([]string{prefix, str}, " ")
	}

	log.Printf("[%s] %s", client.reqID, str)
}

func (client *clientOptions) log(format string, args ...interface{}) {
	if client.nolog == true {
		return
	}

	client.printf("", format, args...)
}

func (client *clientOptions) err(format string, args ...interface{}) {
	client.printf("ERROR:", format, args...)
}
