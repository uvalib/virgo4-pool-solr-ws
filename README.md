# Virgo4 Solr Search Pool Web Service

This is a web service to search a Solr pool for Virgo 4.
It supports the following endpoints:

* GET /version : returns build version
* GET /identify : returns pool information
* GET /healthcheck : returns health check information
* GET /metrics : returns Prometheus metrics
* POST /api/search : returns search results for a Solr pool
* GET /api/resource/{id} : returns detailed information for a single Solr record

### System Requirements

* GO version 1.12.0 or greater
