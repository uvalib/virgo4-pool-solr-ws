package main

import (
	"log"
	"os"
)

type poolConfig struct {
	listenPort           string
	poolType             string
	poolDescription      string
	poolServiceURL       string
	poolLeaders          string
	solrHost             string
	solrCore             string
	solrHandler          string
	solrConnTimeout      string
	solrReadTimeout      string
	solrParameterQt      string
	solrParameterDefType string
	solrParameterFq      string
	solrParameterFl      string
	solrAvailableFacets  string
}

func (cfg *poolConfig) load() {
	cfg.listenPort = os.Getenv("VIRGO4_SOLR_POOL_WS_LISTEN_PORT")
	cfg.poolType = os.Getenv("VIRGO4_SOLR_POOL_WS_POOL_TYPE")
	cfg.poolDescription = os.Getenv("VIRGO4_SOLR_POOL_WS_POOL_DESCRIPTION")
	cfg.poolServiceURL = os.Getenv("VIRGO4_SOLR_POOL_WS_POOL_SERVICE_URL")
	cfg.poolLeaders = os.Getenv("VIRGO4_SOLR_POOL_WS_POOL_LEADERS")
	cfg.solrHost = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_HOST")
	cfg.solrCore = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_CORE")
	cfg.solrHandler = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_HANDLER")
	cfg.solrConnTimeout = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_CONN_TIMEOUT")
	cfg.solrReadTimeout = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_READ_TIMEOUT")
	cfg.solrParameterQt = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_QT")
	cfg.solrParameterDefType = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_DEFTYPE")
	cfg.solrParameterFq = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FQ")
	cfg.solrParameterFl = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FL")
	cfg.solrAvailableFacets = os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_AVAILABLE_FACETS")

	log.Printf("[CONFIG] listenPort           = [%s]", cfg.listenPort)
	log.Printf("[CONFIG] poolType             = [%s]", cfg.poolType)
	log.Printf("[CONFIG] poolDescription      = [%s]", cfg.poolDescription)
	log.Printf("[CONFIG] poolServiceURL       = [%s]", cfg.poolServiceURL)
	log.Printf("[CONFIG] poolLeaders          = [%s]", cfg.poolLeaders)
	log.Printf("[CONFIG] solrHost             = [%s]", cfg.solrHost)
	log.Printf("[CONFIG] solrCore             = [%s]", cfg.solrCore)
	log.Printf("[CONFIG] solrHandler          = [%s]", cfg.solrHandler)
	log.Printf("[CONFIG] solrConnTimeout      = [%s]", cfg.solrConnTimeout)
	log.Printf("[CONFIG] solrReadTimeout      = [%s]", cfg.solrReadTimeout)
	log.Printf("[CONFIG] solrParameterQt      = [%s]", cfg.solrParameterQt)
	log.Printf("[CONFIG] solrParameterDefType = [%s]", cfg.solrParameterDefType)
	log.Printf("[CONFIG] solrParameterFq      = [%s]", cfg.solrParameterFq)
	log.Printf("[CONFIG] solrParameterFl      = [%s]", cfg.solrParameterFl)
	log.Printf("[CONFIG] solrAvailableFacets  = [%s]", cfg.solrAvailableFacets)
}
