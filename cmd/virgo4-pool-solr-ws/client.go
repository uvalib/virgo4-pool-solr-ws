package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var randpool *rand.Rand

// options set by or per client
type clientOptions struct {
	reqID  string // internally generated
	nolog  bool   // internally set
	debug  bool   // client requested
	intuit bool   // client requested
}

func parseBoolOption(opt string, fallback bool) bool {
	var err error
	var val bool

	if val, err = strconv.ParseBool(opt); err != nil {
		val = fallback
	}

	return val
}

func getClientOptions(c *gin.Context) *clientOptions {
	client := clientOptions{}

	client.reqID = randomID()
	client.debug = parseBoolOption(c.Query("debug"), false)
	client.intuit = parseBoolOption(c.Query("intuit"), true)

	query := ""
	if c.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", c.Request.URL.RawQuery)
	}

	client.log("%s %s%s", c.Request.Method, c.Request.URL.Path, query)

	return &client
}

func (c *clientOptions) printf(prefix, format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)

	if prefix != "" {
		str = strings.Join([]string{prefix, str}, " ")
	}

	log.Printf("[%s] %s", c.reqID, str)
}

func (c *clientOptions) log(format string, args ...interface{}) {
	if c.nolog {
		return
	}

	c.printf("", format, args...)
}

func (c *clientOptions) err(format string, args ...interface{}) {
	c.printf("ERROR:", format, args...)
}

func randomID() string {
	return fmt.Sprintf("%0x", randpool.Uint64())
}

func init() {
	randpool = rand.New(rand.NewSource(time.Now().UnixNano()))
}
