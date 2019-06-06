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
	interpoolSearchUrl   configStringItem
	poolType             configStringItem
	poolServiceUrl       configStringItem
	solrHost             configStringItem
	solrCore             configStringItem
	solrHandler          configStringItem
	solrTimeout          configStringItem
	solrParameterQt      configStringItem
	solrParameterDefType configStringItem
	solrParameterFq      configStringItem
	solrParameterFl      configStringItem
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
	flagStringVar(&config.interpoolSearchUrl)
	flagStringVar(&config.poolType)
	flagStringVar(&config.poolServiceUrl)
	flagStringVar(&config.solrHost)
	flagStringVar(&config.solrCore)
	flagStringVar(&config.solrHandler)
	flagStringVar(&config.solrTimeout)
	flagStringVar(&config.solrParameterQt)
	flagStringVar(&config.solrParameterDefType)
	flagStringVar(&config.solrParameterFq)
	flagStringVar(&config.solrParameterFl)

	flag.Parse()

	// check each required option, displaying a warning for empty values.
	// die if any of them are not set
	configOK := true
	configOK = ensureConfigStringSet(&config.listenPort) && configOK
	configOK = ensureConfigStringSet(&config.interpoolSearchUrl) && configOK
	configOK = ensureConfigStringSet(&config.poolType) && configOK
	configOK = ensureConfigStringSet(&config.poolServiceUrl) && configOK
	configOK = ensureConfigStringSet(&config.solrHost) && configOK
	configOK = ensureConfigStringSet(&config.solrCore) && configOK
	configOK = ensureConfigStringSet(&config.solrHandler) && configOK
	configOK = ensureConfigStringSet(&config.solrTimeout) && configOK
	configOK = ensureConfigStringSet(&config.solrParameterQt) && configOK
	configOK = ensureConfigStringSet(&config.solrParameterDefType) && configOK
	configOK = ensureConfigStringSet(&config.solrParameterFq) && configOK
	configOK = ensureConfigStringSet(&config.solrParameterFl) && configOK

	if configOK == false {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("[CONFIG] listenPort           = [%s]", config.listenPort.value)
	log.Printf("[CONFIG] interpoolSearchUrl   = [%s]", config.interpoolSearchUrl.value)
	log.Printf("[CONFIG] poolType             = [%s]", config.poolType.value)
	log.Printf("[CONFIG] poolServiceUrl       = [%s]", config.poolServiceUrl.value)
	log.Printf("[CONFIG] solrHost             = [%s]", config.solrHost.value)
	log.Printf("[CONFIG] solrCore             = [%s]", config.solrCore.value)
	log.Printf("[CONFIG] solrHandler          = [%s]", config.solrHandler.value)
	log.Printf("[CONFIG] solrTimeout          = [%s]", config.solrTimeout.value)
	log.Printf("[CONFIG] solrParameterQt      = [%s]", config.solrParameterQt.value)
	log.Printf("[CONFIG] solrParameterDefType = [%s]", config.solrParameterDefType.value)
	log.Printf("[CONFIG] solrParameterFq      = [%s]", config.solrParameterFq.value)
	log.Printf("[CONFIG] solrParameterFl      = [%s]", config.solrParameterFl.value)
}

func init() {
	config.listenPort = configStringItem{value: "", configItem: configItem{flag: "l", env: "VIRGO4_SOLR_POOL_WS_LISTEN_PORT", desc: "listen port"}}
	config.interpoolSearchUrl = configStringItem{value: "", configItem: configItem{flag: "i", env: "VIRGO4_SOLR_POOL_WS_INTERPOOL_SEARCH_URL", desc: "interpool search url"}}
	config.poolType = configStringItem{value: "", configItem: configItem{flag: "p", env: "VIRGO4_SOLR_POOL_WS_POOL_TYPE", desc: `pool type (e.g. "catalog")`}}
	config.poolServiceUrl = configStringItem{value: "", configItem: configItem{flag: "u", env: "VIRGO4_SOLR_POOL_WS_POOL_SERVICE_URL", desc: "pool service url (registered with interpool search)"}}
	config.solrHost = configStringItem{value: "", configItem: configItem{flag: "h", env: "VIRGO4_SOLR_POOL_WS_SOLR_HOST", desc: `Solr host (e.g. "https://solr.host.lib.virginia.edu:1234/solr")`}}
	config.solrCore = configStringItem{value: "", configItem: configItem{flag: "c", env: "VIRGO4_SOLR_POOL_WS_SOLR_CORE", desc: "Solr core"}}
	config.solrHandler = configStringItem{value: "", configItem: configItem{flag: "s", env: "VIRGO4_SOLR_POOL_WS_SOLR_HANDLER", desc: "Solr search handler"}}
	config.solrTimeout = configStringItem{value: "", configItem: configItem{flag: "t", env: "VIRGO4_SOLR_POOL_WS_SOLR_TIMEOUT", desc: "Solr http client timeout"}}
	config.solrParameterQt = configStringItem{value: "search", configItem: configItem{flag: "w", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_QT", desc: "Solr qt parameter value"}}
	config.solrParameterDefType = configStringItem{value: "lucene", configItem: configItem{flag: "x", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_DEFTYPE", desc: "Solr defType parameter value"}}
	config.solrParameterFq = configStringItem{value: "+shadowed_location_f:VISIBLE", configItem: configItem{flag: "y", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FQ", desc: "Solr fq parameter initial value"}}
	config.solrParameterFl = configStringItem{value: "*,score", configItem: configItem{flag: "z", env: "VIRGO4_SOLR_POOL_WS_SOLR_PARAMETER_FL", desc: "Solr fl parameter value"}}

	getConfigValues()
}
