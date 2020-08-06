package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/uvalib/virgo4-api/v4api"
	"github.com/uvalib/virgo4-jwt/v4jwt"
)

type clientOpts struct {
	debug    bool // controls whether debug info is added to pool results
	verbose  bool // controls whether verbose Solr requests/responses are logged
	ris      bool // controls whether fields are output for client display or RIS export
	citation bool // controls whether fields are output for client display or citation export
}

type clientContext struct {
	reqID       string          // internally generated
	start       time.Time       // internally set
	opts        clientOpts      // options set by client
	claims      *v4jwt.V4Claims // information about this user
	localizer   *i18n.Localizer // per-request localization
	ginCtx      *gin.Context    // gin context
	acceptLang  string          // first language requested by client
	contentLang string          // actual language we are responding with
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
	c.reqID = fmt.Sprintf("%08x", p.randomSource.Uint32())

	// get claims, if any
	if val, ok := ctx.Get("claims"); ok == true {
		c.claims = val.(*v4jwt.V4Claims)
	}

	// determine client preferred language
	c.acceptLang = strings.Split(ctx.GetHeader("Accept-Language"), ",")[0]
	if c.acceptLang == "" {
		c.acceptLang = "en"
	}

	c.localizer = i18n.NewLocalizer(p.translations.bundle, c.acceptLang)

	// kludge to get the response language by checking the tag value returned for a known message ID
	_, tag, _ := c.localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: p.config.Local.Identity.NameXID})
	c.contentLang = tag.String()

	ctx.Header("Content-Language", c.contentLang)

	c.opts.debug = boolOptionWithFallback(ctx.Query("debug"), false)
	c.opts.verbose = boolOptionWithFallback(ctx.Query("verbose"), false)
	c.opts.ris = boolOptionWithFallback(ctx.Query("ris"), false)
	c.opts.citation = boolOptionWithFallback(ctx.Query("citation"), false)
}

func (c *clientContext) logRequest() {
	c.log("------------------------------[ NEW REQUEST ]------------------------------")

	query := ""
	if c.ginCtx.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", c.ginCtx.Request.URL.RawQuery)
	}

	claimsStr := ""
	if c.claims != nil {
		claimsStr = fmt.Sprintf("  [%s; %s; %s; %v]", c.claims.UserID, c.claims.Role, c.claims.AuthMethod, c.claims.IsUVA)
	}

	c.log("[REQUEST] %s %s%s  (%s) => (%s)%s", c.ginCtx.Request.Method, c.ginCtx.Request.URL.Path, query, c.acceptLang, c.contentLang, claimsStr)
}

func (c *clientContext) logResponse(resp searchResponse) {
	msg := fmt.Sprintf("[RESPONSE] status: %d", resp.status)

	if resp.err != nil {
		msg = msg + fmt.Sprintf(", error: %s", resp.err.Error())
	}

	c.log(msg)
}

func (c *clientContext) printf(prefix, format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)

	if prefix != "" {
		str = strings.Join([]string{prefix, str}, " ")
	}

	log.Printf("[%s] %s", c.reqID, str)
}

func (c *clientContext) log(format string, args ...interface{}) {
	c.printf("", format, args...)
}

func (c *clientContext) err(format string, args ...interface{}) {
	c.printf("ERROR:", format, args...)
}

func (c *clientContext) localize(id string) string {
	return c.localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: id})
}

func (c *clientContext) localizedPoolIdentity(p *poolContext) v4api.PoolIdentity {
	id := p.identity

	id.Name = c.localize(id.Name)
	id.Description = c.localize(id.Description)

	for i := range id.SortOptions {
		opt := &id.SortOptions[i]

		opt.Label = c.localize(opt.ID)

		if opt.Asc != "" {
			opt.Asc = c.localize(opt.Asc)
		}

		if opt.Desc != "" {
			opt.Desc = c.localize(opt.Desc)
		}
	}

	return id
}

func (c *clientContext) localizedProviders(p *poolContext) v4api.PoolProviders {
	var providers v4api.PoolProviders

	for _, val := range p.providers.Providers {
		opt := val

		opt.Label = c.localize(opt.Label)

		providers.Providers = append(providers.Providers, opt)
	}

	return providers
}

func (c *clientContext) isAuthenticated() bool {
	if c.claims == nil {
		return false
	}

	return c.claims.IsUVA
}
