# Virgo4 Solr Search Pool Web Service

This is a web service to search a Solr pool for Virgo 4.
It implements portions of the
[Virgo4 Pool Search API](https://github.com/uvalib/v4-api/blob/master/pool-search-api-OAS3.json),
integrating with the
[Virgo4 Interpool Search Service](https://github.com/uvalib/virgo4-search-ws).
It supports the following endpoints:

* GET /version : returns build version
* GET /identify : returns pool information
* GET /healthcheck : returns health check information
* GET /metrics : returns Prometheus metrics
* POST /api/search : returns search results for a Solr pool
* GET /api/resource/{id} : returns detailed information for a single Solr record

### System Requirements

* GO version 1.12.0 or greater
