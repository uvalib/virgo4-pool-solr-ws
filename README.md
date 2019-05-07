# Virgo4 Solr Search Pool Web Service

This is a web service to search a Solr pool for Virgo 4.
It supports the following endpoints:

* GET / or GET /version : returns version information
* GET /healthcheck : returns health check information
* GET /metrics : returns Prometheus metrics
* POST /api/pool_results : returns search results for a Solr pool
* GET /api/pool_results/{id} : returns detailed information for a single Solr record
* POST /api/pool_summary/ : returns a summary of the search results for a Solr pool

### System Requirements

* GO version 1.12.0 or greater
