package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
)

type poolInfo struct {
	name string // pool type
	desc string // localized description
}

// identifying info about the specific type of Solr pool we are
var pool poolInfo

/**
 * Main entry point for the web service
 */
func main() {
	configurePool()

	log.Printf("===> virgo4-pool-solr-ws (%s) starting up <===", pool.name)

	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	router := gin.Default()

	router.Use(cors.Default())

	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)

	router.GET("/favicon.ico", ignoreHandler)

	router.GET("/version", versionHandler)
	router.GET("/identify", identifyHandler)
	router.GET("/healthcheck", healthCheckHandler)

	if api := router.Group("/api"); api != nil {
		api.POST("/search", searchHandler)
		api.GET("/resource/:id", resourceHandler)
	}

	portStr := fmt.Sprintf(":%s", config.listenPort.value)
	log.Printf("Start service on %s", portStr)

	initSolrClient()

	go poolRegistrationLoop()

	log.Fatal(router.Run(portStr))
}

func configurePool() {
	// determine what kind of pool we are

	pool.name = config.poolType.value
	pool.desc = config.poolDescription.value
}
