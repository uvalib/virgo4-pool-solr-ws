package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/uvalib/virgo4-jwt/v4jwt"
)

type clientOpts struct {
	debug   bool // controls whether debug info is added to pool results
	intuit  bool // controls whether intuited (speculative) searches are performed
	verbose bool // controls whether verbose Solr requests/responses are logged
	grouped bool // controls whether Solr results are grouped
}

type clientContext struct {
	reqID     string          // internally generated
	start     time.Time       // internally set
	opts      clientOpts      // options set by client
	claims    *v4jwt.V4Claims  // information about this user
	nolog     bool            // internally set
	localizer *i18n.Localizer // per-request localization
}

func boolOptionWithFallback(opt string, fallback bool) bool {
	var err error
	var val bool

	if val, err = strconv.ParseBool(opt); err != nil {
		val = fallback
	}

	return val
}

func (c *clientContext) init(p *poolContext, ctx *gin.Context) {
	c.start = time.Now()
	c.reqID = fmt.Sprintf("%016x", p.randomSource.Uint64())

	// get claims, if any
	if val, ok := ctx.Get("claims"); ok == true {
		c.claims = val.(*v4jwt.V4Claims)
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

	c.opts.debug = boolOptionWithFallback(ctx.Query("debug"), false)
	c.opts.intuit = false
	c.opts.verbose = boolOptionWithFallback(ctx.Query("verbose"), false)

	if p.config.solrGroupField != "" {
		c.opts.grouped = boolOptionWithFallback(ctx.Query("grouped"), true)
	}

	query := ""
	if ctx.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", ctx.Request.URL.RawQuery)
	}

	c.log("%s %s%s  (%s) => (%s)", ctx.Request.Method, ctx.Request.URL.Path, query, acceptLang, contentLang)
}

func (c *clientContext) printf(prefix, format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)

	if prefix != "" {
		str = strings.Join([]string{prefix, str}, " ")
	}

	log.Printf("[%s] %s", c.reqID, str)
}

func (c *clientContext) log(format string, args ...interface{}) {
	if c.nolog == true {
		return
	}

	c.printf("", format, args...)
}

func (c *clientContext) err(format string, args ...interface{}) {
	c.printf("ERROR:", format, args...)
}

func (c *clientContext) localize(id string) string {
	return c.localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: id})
}

func (c *clientContext) localizedPoolIdentity(p *poolContext) VirgoPoolIdentity {
	id := p.identity

	id.Name = c.localize(p.identity.Name)
	id.Description = c.localize(p.identity.Description)

	return id
}

func (c *clientContext) isAuthenticated() bool {
	if c.claims == nil {
		return false
	}

	return c.claims.IsUVA
}
