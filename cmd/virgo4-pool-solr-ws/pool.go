package main

type poolInfo struct {
	name string // pool type
	desc string // localized description
	url  string // public (service) url
}

// identifying info about the specific type of Solr pool we are
var pool *poolInfo
