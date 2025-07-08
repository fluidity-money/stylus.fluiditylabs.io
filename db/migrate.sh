#!/bin/sh -e

table="stylus_migrations"

dbmate -d migrations --migrations-table "$table" -u "$1" up
