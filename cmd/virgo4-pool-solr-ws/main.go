package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
)

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> virgo4-pool-solr-ws starting up <===")

	cfg := loadConfig()
	pool := initializePool(cfg)

	gin.SetMode(gin.ReleaseMode)
	//gin.DisableConsoleColor()

	router := gin.Default()

	router.Use(gzip.Gzip(gzip.DefaultCompression))

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = true
	corsCfg.AddAllowHeaders("Authorization")
	router.Use(cors.New(corsCfg))

	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)

	router.GET("/favicon.ico", pool.ignoreHandler)

	router.GET("/version", pool.versionHandler)
	router.GET("/identify", pool.identifyHandler)
	router.GET("/healthcheck", pool.healthCheckHandler)

	if api := router.Group("/api"); api != nil {
		api.POST("/search", pool.authenticateHandler, pool.searchHandler)
		api.GET("/resource/:id", pool.authenticateHandler, pool.resourceHandler)
	}

	portStr := fmt.Sprintf(":%s", pool.config.listenPort)
	log.Printf("Start service on %s", portStr)

	log.Fatal(router.Run(portStr))
}
