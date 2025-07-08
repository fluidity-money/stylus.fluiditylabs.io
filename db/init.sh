#!/bin/sh

dbmate \
	-u "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@:5432/${POSTGRES_DB}?sslmode=disable" \
	-d "/usr/local/src/stylus/database" \
	up
