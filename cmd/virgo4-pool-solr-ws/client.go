package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var randpool *rand.Rand

// options set by or per client
type ClientOptions struct {
	reqId string
	debug bool
}

func parseBoolOption(opt string) bool {
	val := false

	if b, err := strconv.ParseBool(opt); err == nil && b == true {
		val = true
	}

	return val
}

func getClientOptions(c *gin.Context) ClientOptions {
	var client ClientOptions

	client.reqId = randomId()
	client.debug = parseBoolOption(c.Query("debug"))

	query := ""
	if c.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", c.Request.URL.RawQuery)
	}

	log.Printf("[%s] %s %s%s", client.reqId, c.Request.Method, c.Request.URL.Path, query)

	return client
}

func randomId() string {
	return fmt.Sprintf("%0x", randpool.Uint64())
}

func init() {
	randpool = rand.New(rand.NewSource(time.Now().UnixNano()))
}
