#!/bin/sh -e

docker build -t fluidity/stylus-database .

docker run \
	-e POSTGRES_USER=${POSTGRES_USER:-superposition} \
	-e POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-superposition} \
	-p 5432:5432 \
	docker.io/fluidity/stylus-database \
	-c log_statement=all
