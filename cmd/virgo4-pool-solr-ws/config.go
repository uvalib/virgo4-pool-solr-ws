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
	listenPort         configStringItem
	interpoolSearchUrl configStringItem
	poolType           configStringItem
	poolServiceUrl     configStringItem
	solrHost           configStringItem
	solrCore           configStringItem
	solrHandler        configStringItem
	solrTimeout        configStringItem
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
	flag.StringVar(&item.value, item.flag, os.Getenv(item.env), item.desc)
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

	if configOK == false {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("[CONFIG] listenPort         = [%s]", config.listenPort.value)
	log.Printf("[CONFIG] interpoolSearchUrl = [%s]", config.interpoolSearchUrl.value)
	log.Printf("[CONFIG] poolType           = [%s]", config.poolType.value)
	log.Printf("[CONFIG] poolServiceUrl     = [%s]", config.poolServiceUrl.value)
	log.Printf("[CONFIG] solrHost           = [%s]", config.solrHost.value)
	log.Printf("[CONFIG] solrCore           = [%s]", config.solrCore.value)
	log.Printf("[CONFIG] solrHandler        = [%s]", config.solrHandler.value)
	log.Printf("[CONFIG] solrTimeout        = [%s]", config.solrTimeout.value)
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

	getConfigValues()
}
