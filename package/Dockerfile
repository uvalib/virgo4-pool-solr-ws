#
# build the target application
#
FROM golang:1.12 as builder
WORKDIR /build
COPY go.mod go.sum Makefile ./
COPY cmd ./cmd
RUN make rebuild-docker

#
# build the targewt container
#
FROM alpine:3.9

# update the packages
RUN apk update && apk upgrade && apk add bash tzdata ca-certificates && rm -rf /var/cache/apk/*

# Create the run user and group
RUN addgroup webservice && adduser webservice -G webservice -D

# set the timezone appropriatly
ENV TZ=UTC
RUN cp /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Specify home 
ENV APP_HOME /virgo4-pool-solr-ws
WORKDIR $APP_HOME

# Create necessary directories
RUN mkdir -p $APP_HOME $APP_HOME/bin
RUN chown -R webservice $APP_HOME && chgrp -R webservice $APP_HOME

# Specify the user
USER webservice

# port and run command
EXPOSE 8080
CMD scripts/entry.sh

# Move in necessary assets
COPY package/data/container_bash_profile /home/webservice/.profile
COPY package/scripts/* $APP_HOME/scripts/
COPY --from=builder /build/bin/virgo4-pool-solr-ws $APP_HOME/bin/

# Add the build tag
#COPY buildtag.* $APP_HOME/

#
# end of file
#
