package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type clientOpts struct {
	debug   bool // controls whether debug info is added to pool results
	intuit  bool // controls whether intuited (speculative) searches are performed
	verbose bool // controls whether verbose Solr requests/responses are logged
	grouped bool // controls whether Solr results are grouped
}

type clientAuth struct {
	token         string // from client headers
	url           string // url for client authentication checks
	checked       bool   // whether we have checked if this token is authenticated
	authenticated bool   // whether this token is authenticated
}

type clientContext struct {
	reqID     string          // internally generated
	start     time.Time       // internally set
	opts      clientOpts      // options set by client
	auth      clientAuth      // authentication-related variables
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

	// get authentication token, if any
	if val, ok := ctx.Get("token"); ok == true {
		c.auth.token = val.(string)
		c.auth.url = fmt.Sprintf("%s/api/authenticated/%s", p.config.clientHost, c.auth.token)
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
	c.opts.intuit = boolOptionWithFallback(ctx.Query("intuit"), true)
	c.opts.verbose = boolOptionWithFallback(ctx.Query("verbose"), false)
	c.opts.grouped = boolOptionWithFallback(ctx.Query("grouped"), true)

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
	id := VirgoPoolIdentity{
		Name:        c.localize(p.identity.Name),
		Summary:     c.localize(p.identity.Summary),
		Description: c.localize(p.identity.Description),
	}

	return id
}

func (c *clientContext) checkAuthentication() error {
	req, reqErr := http.NewRequest("GET", c.auth.url, nil)
	if reqErr != nil {
		c.log("NewRequest() failed: %s", reqErr.Error())
		return fmt.Errorf("Failed to create client authentication check request")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	res, resErr := client.Do(req)

	if resErr != nil {
		c.log("client.Do() failed: %s", resErr.Error())
		return fmt.Errorf("Failed to receive client authentication check response")
	}

	defer res.Body.Close()

	buf, _ := ioutil.ReadAll(res.Body)

	c.log("authentication check returned: %d (%s)", res.StatusCode, buf)

	switch res.StatusCode {
	case http.StatusOK:
		c.auth.authenticated = true
		c.auth.checked = true
	case http.StatusNotFound:
		c.auth.authenticated = false
		c.auth.checked = true
	default:
		return fmt.Errorf("Unexpected status code for client authentication check response: %d", res.StatusCode)
	}

	return nil
}

func (c *clientContext) isAuthenticated() bool {
	if c.auth.token == "" {
		return false
	}

	if strings.HasPrefix(c.auth.url, "http") == false {
		return false
	}

	if c.auth.checked {
		return c.auth.authenticated
	}

	if err := c.checkAuthentication(); err != nil {
		return false
	}

	return c.auth.authenticated
}
