package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

func main() {
	type cfgData struct {
		File   string
		EnvVar string
	}

	var terformBase string
	var tgtEnv string
	var poolName string
	var port string
	flag.StringVar(&terformBase, "dir", "", "local dirctory for virgo4.lib.virginia.edu/ecs-tasks")
	flag.StringVar(&tgtEnv, "env", "staging", "production or staging")
	flag.StringVar(&poolName, "pool", "uva-library", "pool name")
	flag.StringVar(&port, "port", "8080", "port to run the pool on")
	flag.Parse()

	if terformBase == "" {
		log.Fatal("dir is required")
	}
	if tgtEnv != "staging" && tgtEnv != "production" {
		log.Fatal("env must be staging or production")
	}

	cfgBase := path.Join(terformBase, tgtEnv, "pool-solr-ws/environment")

	// This was used to bulk replace xid data in the terraform config
	// pCfg := path.Join(cfgBase, "common/fields.json")
	// pBytes, err := os.ReadFile(pCfg)
	// pString := string(pBytes)
	// mBytes, err := os.ReadFile("f.txt")
	// for _, row := range strings.Split(string(mBytes), "\n") {
	// 	id := strings.Split(string(row), "=")[0]
	// 	label := strings.Split(string(row), "=")[1]
	// 	pString = strings.ReplaceAll(pString, id, label)
	// }

	// cfgF, _ := os.Create(pCfg)
	// cfgF.WriteString(pString)
	// cfgF.Close()
	// log.Fatal("STOP")

	log.Printf("Generate pool config for %s %s from %s", tgtEnv, poolName, cfgBase)
	cfgFiles := []cfgData{
		{File: "common/availability.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_01"},
		{File: "common/providers.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_02"},
		{File: "common/service.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_03"},
		{File: "common/solr.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_04"},
		{File: "common/fields.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_05"},
		{File: "common/sorts.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_06"},
		{File: "common/attributes.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_07"},
		{File: "common/citation_formats.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_08"},
		{File: "common/relators.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_09"},
		{File: "common/publishers.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_10"},
		{File: "common/record_attributes.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_11"},
		{File: "common/copyrights.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_12"},
		{File: "common/titleization.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_13"},
		{File: "common/filters.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_14"},
		{File: "common/resource_types.json", EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_15"},
		{File: fmt.Sprintf("pools/%s.json", poolName), EnvVar: "VIRGO4_SOLR_POOL_WS_JSON_99"},
	}

	fullConfig := ""
	out := make([]string, 0)
	for _, cf := range cfgFiles {
		tgtFile := path.Join(cfgBase, cf.File)
		jsonBytes, err := os.ReadFile(tgtFile)
		if err != nil {
			log.Fatal(err.Error())
		}

		if cf.EnvVar == "VIRGO4_SOLR_POOL_WS_JSON_03" {
			// this is teh service config where the port is set to "8080" overide
			updated := strings.Replace(string(jsonBytes), "8080", port, 1)
			jsonBytes = []byte(updated)
		}

		fullConfig += string(jsonBytes)

		var gzBuf bytes.Buffer
		gz := gzip.NewWriter(&gzBuf)
		_, zErr := gz.Write(jsonBytes)
		if zErr != nil {
			log.Fatal(zErr.Error())
		}
		gz.Close()
		sEnc := base64.StdEncoding.EncodeToString(gzBuf.Bytes())
		out = append(out, fmt.Sprintf("export %s=%s", cf.EnvVar, sEnc))
	}

	outF, err := os.Create("setup_env.sh")
	if err != nil {
		log.Fatal(err.Error())
	}
	outF.WriteString("#!/bin/bash\n\n")
	outF.WriteString(fmt.Sprintf("export VIRGO4_SOLR_POOL_WS_SOLR_HOST=http://virgo4-solr-%s-replica-private.internal.lib.virginia.edu:8080/solr\n", tgtEnv))
	outF.WriteString(fmt.Sprintf("export VIRGO4_SOLR_POOL_WS_DCON_HOST=https://digital-content-metadata-cache-%s.s3.amazonaws.com\n", tgtEnv))
	outF.WriteString(strings.Join(out, "\n"))
	outF.Close()
	os.Chmod("setup_env.sh", 0777)
}
