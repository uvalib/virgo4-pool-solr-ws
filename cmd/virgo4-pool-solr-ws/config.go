package main

import (
	"log"
	"os"
)

type poolConfig struct {
	poolType             string
	poolDescription      string
	poolServiceURL       string
	poolLeaders          string
	listenPort           string
	scoreThresholdMedium string
	scoreThresholdHigh   string
	solrHost             string
	solrCore             string
	solrHandler          string
	solrConnTimeout      string
	solrReadTimeout      string
	solrParameterQt      string
	solrParameterDefType string
	solrParameterFq      string
	solrParameterFl      string
	solrGroupField       string
	solrAvailableFacets  string
}

func ensureSet(env string) string {
	val, set := os.LookupEnv(env)

	if set == false {
		log.Printf("environment variable not set: [%s]", env)
		os.Exit(1)
	}

	return val
}

func ensureSetAndNonEmpty(env string) string {
	val := ensureSet(env)

	if val == "" {
		log.Printf("environment variable not set: [%s]", env)
		os.Exit(1)
	}

	return val
}

func (cfg *poolConfig) load() {
	cfg.poolType = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_POOL_TYPE")
	cfg.poolDescription = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_POOL_DESCRIPTION")
	cfg.poolServiceURL = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_POOL_SERVICE_URL")
	cfg.poolLeaders = ensureSet("VIRGO4_SOLR_POOL_WS_POOL_LEADERS")
	cfg.listenPort = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_LISTEN_PORT")
	cfg.scoreThresholdMedium = ensureSet("VIRGO4_SOLR_POOL_WS_SCORE_THRESHOLD_MEDIUM")
	cfg.scoreThresholdHigh = ensureSet("VIRGO4_SOLR_POOL_WS_SCORE_THRESHOLD_HIGH")
	cfg.solrHost = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_HOST")
	cfg.solrCore = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_CORE")
	cfg.solrHandler = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_HANDLER")
	cfg.solrConnTimeout = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_CONN_TIMEOUT")
	cfg.solrReadTimeout = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_READ_TIMEOUT")
	cfg.solrParameterQt = ensureSet("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_QT")
	cfg.solrParameterDefType = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_DEFTYPE")
	cfg.solrParameterFq = ensureSet("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FQ")
	cfg.solrParameterFl = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FL")
	cfg.solrGroupField = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_GROUP_FIELD")
	cfg.solrAvailableFacets = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_AVAILABLE_FACETS")

	log.Printf("[CONFIG] poolType             = [%s]", cfg.poolType)
	log.Printf("[CONFIG] poolDescription      = [%s]", cfg.poolDescription)
	log.Printf("[CONFIG] poolServiceURL       = [%s]", cfg.poolServiceURL)
	log.Printf("[CONFIG] poolLeaders          = [%s]", cfg.poolLeaders)
	log.Printf("[CONFIG] listenPort           = [%s]", cfg.listenPort)
	log.Printf("[CONFIG] scoreThresholdMedium = [%s]", cfg.scoreThresholdMedium)
	log.Printf("[CONFIG] scoreThresholdHigh   = [%s]", cfg.scoreThresholdHigh)
	log.Printf("[CONFIG] solrHost             = [%s]", cfg.solrHost)
	log.Printf("[CONFIG] solrCore             = [%s]", cfg.solrCore)
	log.Printf("[CONFIG] solrHandler          = [%s]", cfg.solrHandler)
	log.Printf("[CONFIG] solrConnTimeout      = [%s]", cfg.solrConnTimeout)
	log.Printf("[CONFIG] solrReadTimeout      = [%s]", cfg.solrReadTimeout)
	log.Printf("[CONFIG] solrParameterQt      = [%s]", cfg.solrParameterQt)
	log.Printf("[CONFIG] solrParameterDefType = [%s]", cfg.solrParameterDefType)
	log.Printf("[CONFIG] solrParameterFq      = [%s]", cfg.solrParameterFq)
	log.Printf("[CONFIG] solrParameterFl      = [%s]", cfg.solrParameterFl)
	log.Printf("[CONFIG] solrGroupField       = [%s]", cfg.solrGroupField)
	log.Printf("[CONFIG] solrAvailableFacets  = [%s]", cfg.solrAvailableFacets)
}
