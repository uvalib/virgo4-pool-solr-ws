# Virgo4 Solr Search Pool setup helper

This is a command line helper utility that will parse the terraform config files and generate
a file named setup_env.sh in the directory the command is run from. That file contains all
of the enviroment exports needed to configure the service.

To run from check directory:
`go run setup/*.go -dir {terrform staging pool env dir} -env {staging | production} -pool {hathitrust | images | uva-library} -port {service port}`

Note: if not set, env defaults to staging and pool defaults to uva-library

When done, source the generated file `. ./setup_env.sh` and launch the pool with `go run cmd/*.go`
