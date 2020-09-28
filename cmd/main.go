package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> virgo4-pool-solr-ws starting up <===")

	cfg := loadConfig()
	pool := initializePool(cfg)

	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	router := gin.Default()

	router.Use(gzip.Gzip(gzip.DefaultCompression))

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = true
	corsCfg.AddAllowHeaders("Authorization")
	router.Use(cors.New(corsCfg))

	p := ginprometheus.NewPrometheus("gin")

	// roundabout setup of /metrics endpoint to avoid double-gzip of response
	router.Use(p.HandlerFunc())
	h := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{DisableCompression: true}))

	router.GET(p.MetricsPath, func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	})

	router.GET("/favicon.ico", pool.ignoreHandler)

	router.GET("/version", pool.versionHandler)
	router.GET("/identify", pool.identifyHandler)
	router.GET("/healthcheck", pool.healthCheckHandler)

	if api := router.Group("/api", pool.authenticateHandler); api != nil {
		api.POST("/search", pool.searchHandler)
		api.POST("/search/facets", pool.facetsHandler)
		api.GET("/resource/:id", pool.resourceHandler)
		api.GET("/providers", pool.providersHandler)
		api.POST("/shelf-browse", pool.shelfBrowseHandler)
	}

	if admin := router.Group("/admin", pool.authenticateHandler, pool.adminHandler); admin != nil {
		pprof.RouteRegister(admin, "pprof")
	}

	router.Use(static.Serve("/assets", static.LocalFile("./assets", false)))

	portStr := fmt.Sprintf(":%s", pool.config.Global.Service.Port)
	log.Printf("[MAIN] listening on %s", portStr)

	log.Fatal(router.Run(portStr))
}
