
.PHONY: build

build: chains.tsv web

chains.tsv: ../chains.tsv
	@cp ../chains.tsv chains.tsv

web: $(shell find . -name '*.go' -or -name '*.clj' -or -name '*.html' -or -name '*.tmpl')
	@go build

bootstrap: web
	@cp web bootstrap

bootstrap.zip: bootstrap
	@zip bootstrap.zip bootstrap
