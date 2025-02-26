package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
)

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> virgo4-pool-solr-ws starting up <===")

	var cfg *poolConfig
	var cfgFile string
	flag.StringVar(&cfgFile, "cfg", "", "local cfg file")
	flag.Parse()
	if cfgFile != "" {
		log.Printf("===> load config from: %s", cfgFile)
		jsonBytes, err := os.ReadFile(cfgFile)
		if err != nil {
			log.Fatal(err.Error())
		}
		var pc poolConfig
		cErr := json.Unmarshal(jsonBytes, &pc)
		if cErr != nil {
			log.Fatal(cErr)
		}
		cfg = &pc
	} else {
		log.Printf("===> load config from environment")
		cfg = loadConfig()
	}

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

	//
	// we are removing Prometheus support for now
	//
	//p := ginprometheus.NewPrometheus("gin")

	// roundabout setup of /metrics endpoint to avoid double-gzip of response
	//router.Use(p.HandlerFunc())
	//h := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{DisableCompression: true}))

	//router.GET(p.MetricsPath, func(c *gin.Context) {
	//	h.ServeHTTP(c.Writer, c.Request)
	//})

	router.GET("/favicon.ico", pool.ignoreHandler)

	router.GET("/version", pool.versionHandler)
	router.GET("/identify", pool.identifyHandler)
	router.GET("/healthcheck", pool.healthCheckHandler)

	if api := router.Group("/api"); api != nil {
		api.POST("/search", pool.authenticateHandler, pool.searchHandler)
		api.POST("/search/facets", pool.authenticateHandler, pool.facetsHandler)
		api.GET("/resource/:id", pool.authenticateHandler, pool.resourceHandler)
		api.GET("/providers", pool.providersHandler) // No auth needed here
		api.GET("/filters", pool.authenticateHandler, pool.filtersHandler)
	}

	if admin := router.Group("/admin", pool.authenticateHandler, pool.adminHandler); admin != nil {
		pprof.RouteRegister(admin, "pprof")
	}

	router.Use(static.Serve("/assets", static.LocalFile("./assets", false)))

	portStr := fmt.Sprintf(":%s", pool.config.Global.Service.Port)
	log.Printf("[MAIN] listening on %s", portStr)

	log.Fatal(router.Run(portStr))
}
