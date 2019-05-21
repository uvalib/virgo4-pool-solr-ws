package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
)

const program = "Virgo4 Solr Pool Search"

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> %s starting up <===", program)

	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	router := gin.Default()

	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)

	router.GET("/favicon.ico", ignoreHandler)

	router.GET("/version", versionHandler)

	router.GET("/healthcheck", healthCheckHandler)

	api := router.Group("/api")

	api.POST("/search", searchHandler)
	api.GET("/search/:id", recordHandler)
	api.POST("/pool_summary", poolSummaryHandler)

	portStr := fmt.Sprintf(":%s", config.listenPort.value)
	log.Printf("Start service on %s", portStr)

	log.Fatal(router.Run(portStr))
}
