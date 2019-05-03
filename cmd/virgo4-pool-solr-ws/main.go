package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

const program = "Virgo4 Solr Pool Search"
const version = "0.1"

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> %s starting up <===", program)

	gin.SetMode(gin.ReleaseMode)
	//gin.DisableConsoleColor()

	router := gin.Default()

	router.GET("/favicon.ico", ignoreHandler)

	router.GET("/", versionHandler)
	router.GET("/version", versionHandler)

	router.GET("/healthcheck", healthCheckHandler)
	router.GET("/metrics", metricsHandler)

	api := router.Group("/api")

	api.POST("/pool_results", poolResultsHandler)
	api.GET("/pool_results/:id", poolResultsRecordHandler)
	api.POST("/pool_summary", poolSummaryHandler)

	portStr := fmt.Sprintf(":%s", config.listenPort.value)
	log.Printf("Start service on %s", portStr)

	log.Fatal(router.Run(portStr))
}
