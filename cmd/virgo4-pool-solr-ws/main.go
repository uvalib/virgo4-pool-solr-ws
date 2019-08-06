package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
)

// FIXME: temp during migration
var ctx poolContext

/**
 * Main entry point for the web service
 */
func main() {
	config := poolConfig{}
	config.load()

	ctx = poolContext{}
	ctx.init(&config)

	log.Printf("===> virgo4-pool-solr-ws (%s) starting up <===", ctx.identity.Name)

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

	router.GET("/favicon.ico", ctx.ignoreHandler)

	router.GET("/version", ctx.versionHandler)
	router.GET("/identify", ctx.identifyHandler)
	router.GET("/healthcheck", ctx.healthCheckHandler)

	if api := router.Group("/api"); api != nil {
		api.POST("/search", ctx.authenticateHandler, ctx.searchHandler)
		api.GET("/resource/:id", ctx.authenticateHandler, ctx.resourceHandler)
	}

	portStr := fmt.Sprintf(":%s", ctx.config.listenPort)
	log.Printf("Start service on %s", portStr)

	log.Fatal(router.Run(portStr))
}
