package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// options set by or per client
type clientOptions struct {
	start         time.Time       // internally set
	reqID         string          // internally generated
	token         string          // internally set
	authenticated bool            // internally set based on token, if needed
	localizer     *i18n.Localizer // for localization
	nolog         bool            // internally set
	debug         bool            // client requested -- controls whether debug info is added to pool results
	intuit        bool            // client requested -- controls whether intuited (speculative) searches are performed
	verbose       bool            // client requested -- controls whether verbose Solr requests/responses are logged
	grouped       bool            // client requested -- controls whether Solr results are grouped
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
	c.reqID = fmt.Sprintf("%016x", p.randomSource.Uint64())

	// get authentication token, if any
	if val, ok := ctx.Get("token"); ok == true {
		c.token = val.(string)
	}

	// determine client preferred language
	acceptLang := strings.Split(ctx.GetHeader("Accept-Language"), ",")[0]
	if acceptLang == "" {
		acceptLang = "en"
	}

	c.localizer = i18n.NewLocalizer(p.translations.bundle, acceptLang)

	// kludge to get the response language by checking the tag value returned for a known message ID
	_, tag, _ := c.localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: p.translations.messageIDs[0]})
	contentLang := tag.String()

	ctx.Header("Content-Language", contentLang)

	c.debug = boolOptionWithFallback(ctx.Query("debug"), false)
	c.intuit = boolOptionWithFallback(ctx.Query("intuit"), true)
	c.verbose = boolOptionWithFallback(ctx.Query("verbose"), false)
	c.grouped = boolOptionWithFallback(ctx.Query("grouped"), false)

	query := ""
	if ctx.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", ctx.Request.URL.RawQuery)
	}

	c.log("%s %s%s  (%s) => (%s)", ctx.Request.Method, ctx.Request.URL.Path, query, acceptLang, contentLang)
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
	return c.localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: id})
}

func (c *clientOptions) localizedPoolIdentity(p *poolContext) VirgoPoolIdentity {
	id := VirgoPoolIdentity{
		Name:        c.localize(p.identity.Name),
		Summary:     c.localize(p.identity.Summary),
		Description: c.localize(p.identity.Description),
	}

	return id
}
