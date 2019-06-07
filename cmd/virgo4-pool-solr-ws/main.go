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

	// FIXME: reinstated until terraform handles new pool-specific env vars
	// (when removing this, also remove default description in config.go)
	switch pool.name {
	case "catalog":
		pool.desc = "The UVA Library Catalog"
		config.poolLeaders.value = "+leader67_f:(am OR tm)"

	case "catalog_broad":
		pool.desc = "The UVA Library Broad Catalog"
		config.poolLeaders.value = "+leader67_f:(am OR tm OR aa OR mm OR ai OR em)"

	case "serials":
		pool.desc = "The UVA Library Serials Catalog"
		config.poolLeaders.value = "+leader67_f:(as)"

	case "music_recordings":
		pool.desc = "The UVA Library Music Recordings Catalog"
		config.poolLeaders.value = "+leader67_f:(jm)"

	case "sound_recordings":
		pool.desc = "The UVA Library Sound Recordings Catalog"
		config.poolLeaders.value = "+leader67_f:(im)"

	case "video":
		pool.desc = "The UVA Library Video Catalog"
		config.poolLeaders.value = "+leader67_f:(gm)"

	case "musical_scores":
		pool.desc = "The UVA Library Musical Scores Catalog"
		config.poolLeaders.value = "+leader67_f:(cm OR dm)"

	case "archival":
		pool.desc = "The UVA Library Archival Catalog"
		config.poolLeaders.value = "+leader67_f:(pc OR tc OR ac)"

	default:
		log.Fatalf("Unhandled pool type: [%s]", pool.name)
	}
}
