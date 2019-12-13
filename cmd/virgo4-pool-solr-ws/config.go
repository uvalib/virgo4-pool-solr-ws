package main

import (
	"log"
	"os"
)

type poolConfig struct {
	poolName              string
	poolDescription       string
	poolDefinition        string
	poolFacets            string
	listenPort            string
	clientHost            string
	sirsiURLTemplate      string
	coverImageURLTemplate string
	scoreThresholdMedium  string
	scoreThresholdHigh    string
	solrHost              string
	solrCore              string
	solrHandler           string
	solrConnTimeout       string
	solrReadTimeout       string
	solrParameterQt       string
	solrParameterDefType  string
	solrParameterFq       string
	solrParameterFl       string
	solrGroupField        string
	solrFacetManifest     string
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

func loadConfig() *poolConfig {
	cfg := poolConfig{}

	cfg.poolName = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_POOL_NAME")
	cfg.poolDescription = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_POOL_DESCRIPTION")
	cfg.poolDefinition = ensureSet("VIRGO4_SOLR_POOL_WS_POOL_DEFINITION")
	cfg.poolFacets = ensureSet("VIRGO4_SOLR_POOL_WS_POOL_FACETS")
	cfg.listenPort = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_LISTEN_PORT")
	cfg.clientHost = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_CLIENT_HOST")
	cfg.sirsiURLTemplate = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SIRSI_URL_TEMPLATE")
	cfg.coverImageURLTemplate = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_COVER_IMAGE_URL_TEMPLATE")
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
	cfg.solrFacetManifest = ensureSetAndNonEmpty("VIRGO4_SOLR_POOL_WS_SOLR_FACET_MANIFEST")

	log.Printf("[CONFIG] poolName              = [%s]", cfg.poolName)
	log.Printf("[CONFIG] poolDescription       = [%s]", cfg.poolDescription)
	log.Printf("[CONFIG] poolDefinition        = [%s]", cfg.poolDefinition)
	log.Printf("[CONFIG] poolFacets            = [%s]", cfg.poolFacets)
	log.Printf("[CONFIG] listenPort            = [%s]", cfg.listenPort)
	log.Printf("[CONFIG] clientHost            = [%s]", cfg.clientHost)
	log.Printf("[CONFIG] sirsiURLTemplate      = [%s]", cfg.sirsiURLTemplate)
	log.Printf("[CONFIG] coverImageURLTemplate = [%s]", cfg.coverImageURLTemplate)
	log.Printf("[CONFIG] scoreThresholdMedium  = [%s]", cfg.scoreThresholdMedium)
	log.Printf("[CONFIG] scoreThresholdHigh    = [%s]", cfg.scoreThresholdHigh)
	log.Printf("[CONFIG] solrHost              = [%s]", cfg.solrHost)
	log.Printf("[CONFIG] solrCore              = [%s]", cfg.solrCore)
	log.Printf("[CONFIG] solrHandler           = [%s]", cfg.solrHandler)
	log.Printf("[CONFIG] solrConnTimeout       = [%s]", cfg.solrConnTimeout)
	log.Printf("[CONFIG] solrReadTimeout       = [%s]", cfg.solrReadTimeout)
	log.Printf("[CONFIG] solrParameterQt       = [%s]", cfg.solrParameterQt)
	log.Printf("[CONFIG] solrParameterDefType  = [%s]", cfg.solrParameterDefType)
	log.Printf("[CONFIG] solrParameterFq       = [%s]", cfg.solrParameterFq)
	log.Printf("[CONFIG] solrParameterFl       = [%s]", cfg.solrParameterFl)
	log.Printf("[CONFIG] solrGroupField        = [%s]", cfg.solrGroupField)
	log.Printf("[CONFIG] solrFacetManifest     = [%s]", cfg.solrFacetManifest)

	return &cfg
}
