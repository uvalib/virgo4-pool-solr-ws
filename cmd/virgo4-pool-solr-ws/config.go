package main

import (
	"flag"
	"log"
	"os"
	"strconv"
)

type configItem struct {
	flag string
	env  string
	desc string
}

type configStringItem struct {
	value string
	configItem
}

type configBoolItem struct {
	value bool
	configItem
}

type configData struct {
	listenPort           configStringItem
	poolType             configStringItem
	poolDescription      configStringItem
	poolServiceURL       configStringItem
	poolLeaders          configStringItem
	solrHost             configStringItem
	solrCore             configStringItem
	solrHandler          configStringItem
	solrConnTimeout      configStringItem
	solrReadTimeout      configStringItem
	solrParameterQt      configStringItem
	solrParameterDefType configStringItem
	solrParameterFq      configStringItem
	solrParameterFl      configStringItem
	solrAvailableFacets  configStringItem
}

var config configData

func getBoolEnv(optEnv string) bool {
	value, _ := strconv.ParseBool(os.Getenv(optEnv))

	return value
}

func ensureConfigStringSet(item *configStringItem) bool {
	isSet := true

	if item.value == "" {
		isSet = false
		log.Printf("[ERROR] %s is not set, use %s variable or -%s flag", item.desc, item.env, item.flag)
	}

	return isSet
}

func flagStringVar(item *configStringItem) {
	if val, set := os.LookupEnv(item.env); set == true {
		flag.StringVar(&item.value, item.flag, val, item.desc)
	}
}

func flagBoolVar(item *configBoolItem) {
	flag.BoolVar(&item.value, item.flag, getBoolEnv(item.env), item.desc)
}

func getConfigValues() {
	// get values from the command line first, falling back to environment variables
	flagStringVar(&config.listenPort)
	flagStringVar(&config.poolType)
	flagStringVar(&config.poolDescription)
	flagStringVar(&config.poolServiceURL)
	flagStringVar(&config.poolLeaders)
	flagStringVar(&config.solrHost)
	flagStringVar(&config.solrCore)
	flagStringVar(&config.solrHandler)
	flagStringVar(&config.solrConnTimeout)
	flagStringVar(&config.solrReadTimeout)
	flagStringVar(&config.solrParameterQt)
	flagStringVar(&config.solrParameterDefType)
	flagStringVar(&config.solrParameterFq)
	flagStringVar(&config.solrParameterFl)
	flagStringVar(&config.solrAvailableFacets)

	flag.Parse()

	// check each required option, displaying a warning for empty values.
	// die if any of them are not set
	configOK := true
	configOK = ensureConfigStringSet(&config.listenPort) && configOK
	configOK = ensureConfigStringSet(&config.poolType) && configOK
	configOK = ensureConfigStringSet(&config.poolDescription) && configOK
	configOK = ensureConfigStringSet(&config.poolServiceURL) && configOK
	//configOK = ensureConfigStringSet(&config.poolLeaders) && configOK
	configOK = ensureConfigStringSet(&config.solrHost) && configOK
	configOK = ensureConfigStringSet(&config.solrCore) && configOK
	configOK = ensureConfigStringSet(&config.solrHandler) && configOK
	configOK = ensureConfigStringSet(&config.solrConnTimeout) && configOK
	configOK = ensureConfigStringSet(&config.solrReadTimeout) && configOK
	//configOK = ensureConfigStringSet(&config.solrParameterQt) && configOK
	configOK = ensureConfigStringSet(&config.solrParameterDefType) && configOK
	//configOK = ensureConfigStringSet(&config.solrParameterFq) && configOK
	configOK = ensureConfigStringSet(&config.solrParameterFl) && configOK
	configOK = ensureConfigStringSet(&config.solrAvailableFacets) && configOK

	if configOK == false {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("[CONFIG] listenPort           = [%s]", config.listenPort.value)
	log.Printf("[CONFIG] poolType             = [%s]", config.poolType.value)
	log.Printf("[CONFIG] poolDescription      = [%s]", config.poolDescription.value)
	log.Printf("[CONFIG] poolServiceURL       = [%s]", config.poolServiceURL.value)
	log.Printf("[CONFIG] poolLeaders          = [%s]", config.poolLeaders.value)
	log.Printf("[CONFIG] solrHost             = [%s]", config.solrHost.value)
	log.Printf("[CONFIG] solrCore             = [%s]", config.solrCore.value)
	log.Printf("[CONFIG] solrHandler          = [%s]", config.solrHandler.value)
	log.Printf("[CONFIG] solrConnTimeout      = [%s]", config.solrConnTimeout.value)
	log.Printf("[CONFIG] solrReadTimeout      = [%s]", config.solrReadTimeout.value)
	log.Printf("[CONFIG] solrParameterQt      = [%s]", config.solrParameterQt.value)
	log.Printf("[CONFIG] solrParameterDefType = [%s]", config.solrParameterDefType.value)
	log.Printf("[CONFIG] solrParameterFq      = [%s]", config.solrParameterFq.value)
	log.Printf("[CONFIG] solrParameterFl      = [%s]", config.solrParameterFl.value)
	log.Printf("[CONFIG] solrAvailableFacets  = [%s]", config.solrAvailableFacets.value)
}

func init() {
	config.listenPort = configStringItem{value: "", configItem: configItem{flag: "l", env: "VIRGO4_SOLR_POOL_WS_LISTEN_PORT", desc: "listen port"}}
	config.poolType = configStringItem{value: "", configItem: configItem{flag: "p", env: "VIRGO4_SOLR_POOL_WS_POOL_TYPE", desc: `pool type (e.g. "catalog")`}}
	config.poolDescription = configStringItem{value: "", configItem: configItem{flag: "d", env: "VIRGO4_SOLR_POOL_WS_POOL_DESCRIPTION", desc: `pool description (e.g. "The UVA Library Catalog")`}}
	config.poolServiceURL = configStringItem{value: "", configItem: configItem{flag: "u", env: "VIRGO4_SOLR_POOL_WS_POOL_SERVICE_URL", desc: "pool service url (reported to client)"}}
	config.poolLeaders = configStringItem{value: "", configItem: configItem{flag: "e", env: "VIRGO4_SOLR_POOL_WS_POOL_LEADERS", desc: `pool leaders (appended to Solr fq parameter)`}}
	config.solrHost = configStringItem{value: "", configItem: configItem{flag: "h", env: "VIRGO4_SOLR_POOL_WS_SOLR_HOST", desc: `Solr host (e.g. "https://solr.host.lib.virginia.edu:1234/solr")`}}
	config.solrCore = configStringItem{value: "", configItem: configItem{flag: "c", env: "VIRGO4_SOLR_POOL_WS_SOLR_CORE", desc: "Solr core"}}
	config.solrHandler = configStringItem{value: "", configItem: configItem{flag: "s", env: "VIRGO4_SOLR_POOL_WS_SOLR_HANDLER", desc: "Solr search handler"}}
	config.solrConnTimeout = configStringItem{value: "", configItem: configItem{flag: "t", env: "VIRGO4_SOLR_POOL_WS_SOLR_CONN_TIMEOUT", desc: "Solr http connection/TLS handshake timeout"}}
	config.solrReadTimeout = configStringItem{value: "", configItem: configItem{flag: "r", env: "VIRGO4_SOLR_POOL_WS_SOLR_READ_TIMEOUT", desc: "Solr http read timeout"}}
	config.solrParameterQt = configStringItem{value: "", configItem: configItem{flag: "w", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_QT", desc: "Solr qt parameter value"}}
	config.solrParameterDefType = configStringItem{value: "", configItem: configItem{flag: "x", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_DEFTYPE", desc: "Solr defType parameter value"}}
	config.solrParameterFq = configStringItem{value: "", configItem: configItem{flag: "y", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FQ", desc: "Solr fq parameter initial value"}}
	config.solrParameterFl = configStringItem{value: "", configItem: configItem{flag: "z", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FL", desc: "Solr fl parameter value"}}
	config.solrAvailableFacets = configStringItem{value: "", configItem: configItem{flag: "f", env: "VIRGO4_SOLR_POOL_WS_SOLR_AVAILABLE_FACETS", desc: "facets exposed to the client"}}

	getConfigValues()
}
