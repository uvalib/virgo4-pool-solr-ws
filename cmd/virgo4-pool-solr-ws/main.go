package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
)

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> virgo4-pool-solr-ws (%s) starting up <===", pool.name)

	gin.SetMode(gin.ReleaseMode)
	//gin.DisableConsoleColor()

	router := gin.Default()

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = true
	corsCfg.AddAllowHeaders("Authorization")
	router.Use(cors.New(corsCfg))

	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)

	router.GET("/favicon.ico", ignoreHandler)

	router.GET("/version", versionHandler)
	router.GET("/identify", identifyHandler)
	router.GET("/healthcheck", healthCheckHandler)

	if api := router.Group("/api"); api != nil {
		api.POST("/search", authenticateHandler, searchHandler)
		api.GET("/resource/:id", authenticateHandler, resourceHandler)
	}

	portStr := fmt.Sprintf(":%s", config.listenPort.value)
	log.Printf("Start service on %s", portStr)

	initSolrClient()

	log.Fatal(router.Run(portStr))
}
