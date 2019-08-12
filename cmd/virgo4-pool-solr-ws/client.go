package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// options set by or per client
type clientOptions struct {
	start       time.Time        // internally set
	reqID       string           // internally generated
	acceptLang  string           // set based on client header
	contentLang language.Tag     // set based on what the pool supports
	printer     *message.Printer // for localization
	nolog       bool             // internally set
	debug       bool             // client requested -- controls whether debug info is added to pool results
	intuit      bool             // client requested -- controls whether intuited (speculative) searches are performed
	verbose     bool             // client requested -- controls whether verbose Solr requests/responses are logged
	grouped     bool             // client requested -- controls whether Solr results are grouped
}

func boolOptionWithFallback(opt string, fallback bool) bool {
	var err error
	var val bool

	if val, err = strconv.ParseBool(opt); err != nil {
		val = fallback
	}

	return val
}

func (c *clientOptions) init(p *poolContext, ctx *gin.Context) {
	c.start = time.Now()
	c.reqID = fmt.Sprintf("%0x", p.randomSource.Uint64())

	if c.acceptLang = ctx.GetHeader("Accept-Language"); c.acceptLang == "" {
		c.acceptLang = "en-US"
	}

	c.contentLang = message.MatchLanguage(c.acceptLang)

	c.printer = message.NewPrinter(c.contentLang)

	ctx.Header("Content-Language", c.contentLang.String())

	c.debug = boolOptionWithFallback(ctx.Query("debug"), false)
	c.intuit = boolOptionWithFallback(ctx.Query("intuit"), true)
	c.verbose = boolOptionWithFallback(ctx.Query("verbose"), false)
	c.grouped = boolOptionWithFallback(ctx.Query("grouped"), false)

	query := ""
	if ctx.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", ctx.Request.URL.RawQuery)
	}

	c.log("%s %s%s", ctx.Request.Method, ctx.Request.URL.Path, query)
}

func (c *clientOptions) printf(prefix, format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)

	if prefix != "" {
		str = strings.Join([]string{prefix, str}, " ")
	}

	log.Printf("[%s] %s", c.reqID, str)
}

func (c *clientOptions) log(format string, args ...interface{}) {
	if c.nolog == true {
		return
	}

	c.printf("", format, args...)
}

func (c *clientOptions) err(format string, args ...interface{}) {
	c.printf("ERROR:", format, args...)
}

func (c *clientOptions) localize(id string) string {
	return c.printer.Sprintf(id)
}
