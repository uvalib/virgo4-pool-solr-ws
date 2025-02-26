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
	debug    bool // controls whether debug info is added to pool results
	verbose  bool // controls whether verbose Solr requests/responses are logged
	citation bool // controls whether fields are output for client display or citation export
	snippets bool // controls whether search results are augmented with highlighted search snippets
}

type clientContext struct {
	reqID        string          // internally generated
	ip           string          // from gin context
	tokenSnippet string          // unique-ish snippet of user token
	start        time.Time       // internally set
	opts         clientOpts      // options set by client
	claims       *v4jwt.V4Claims // information about this user
	localizer    *i18n.Localizer // per-request localization
	ginCtx       *gin.Context    // gin context
	acceptLang   string          // first language requested by client
	contentLang  string          // actual language we are responding with
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
	c.ginCtx = ctx

	c.start = time.Now()
	c.reqID = "internal"
	c.ip = "internal"
	c.tokenSnippet = "internal"
	c.acceptLang = "en-US"

	// if there is no gin context, wrap up and return
	if ctx == nil {
		c.localizer = i18n.NewLocalizer(p.translations.bundle, c.acceptLang)
		return
	}

	// configure remaining items based on data in the gin context

	c.reqID = fmt.Sprintf("%08x", p.randomSource.Uint32())
	c.ip = ctx.ClientIP()

	// get token, if any, and use the last bits for logging
	c.tokenSnippet = "no_token"
	if val, ok := ctx.Get("token"); ok == true {
		str := "--------" + val.(string)
		str = str[len(str)-8:]
		c.tokenSnippet = str
	}

	// get claims, if any
	if val, ok := ctx.Get("claims"); ok == true {
		c.claims = val.(*v4jwt.V4Claims)
	}

	// determine client preferred language
	acceptLang := strings.Split(ctx.GetHeader("Accept-Language"), ",")[0]
	if acceptLang != "" {
		c.acceptLang = acceptLang
	}

	c.localizer = i18n.NewLocalizer(p.translations.bundle, c.acceptLang)

	c.opts.debug = boolOptionWithFallback(ctx.Query("debug"), false)
	c.opts.verbose = boolOptionWithFallback(ctx.Query("verbose"), false)
	c.opts.citation = boolOptionWithFallback(ctx.Query("citation"), false)
	c.opts.snippets = boolOptionWithFallback(ctx.Query("snippets"), true)
}

func (c *clientContext) logRequest() {
	query := ""
	if c.ginCtx.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", c.ginCtx.Request.URL.RawQuery)
	}

	claimsStr := ""
	if c.claims != nil {
		claimsStr = fmt.Sprintf("  [%s; %s; %s; %v]", c.claims.UserID, c.claims.Role, c.claims.AuthMethod, c.claims.IsUVA)
	}

	c.log("REQUEST: %s %s%s  (%s) => (%s)%s", c.ginCtx.Request.Method, c.ginCtx.Request.URL.Path, query, c.acceptLang, c.contentLang, claimsStr)
}

func (c *clientContext) logResponse(resp searchResponse) {
	msg := fmt.Sprintf("RESPONSE: status: %d", resp.status)

	if resp.err != nil {
		msg = msg + fmt.Sprintf(", error: %s", resp.err.Error())
	}

	c.log(msg)
}

func (c *clientContext) log(format string, args ...interface{}) {
	parts := []string{
		fmt.Sprintf("[ip:%s]", c.ip),
		fmt.Sprintf("[req:%s]", c.reqID),
		fmt.Sprintf("[tok:%s]", c.tokenSnippet),
		fmt.Sprintf(format, args...),
	}

	log.Printf("%s", strings.Join(parts, " "))
}

func (c *clientContext) warn(format string, args ...interface{}) {
	c.log("WARNING: "+format, args...)
}

func (c *clientContext) err(format string, args ...interface{}) {
	c.log("ERROR: "+format, args...)
}

func (c *clientContext) verbose(format string, args ...interface{}) {
	if c.opts.verbose == false {
		return
	}

	c.log("VERBOSE: "+format, args...)
}

func (c *clientContext) localize(id string) string {
	val, err := c.localizer.Localize(&i18n.LocalizeConfig{MessageID: id})
	if err != nil {
		return id
	}
	return val
}

func (c *clientContext) isAuthenticated() bool {
	if c.claims == nil {
		return false
	}

	return c.claims.IsUVA
}
