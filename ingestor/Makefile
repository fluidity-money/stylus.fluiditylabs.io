
.PHONY: build

build: chains.tsv ingestor

chains.tsv: ../chains.tsv
	@cp ../chains.tsv chains.tsv

ingestor: $(shell find . -name '*.go')
	@go build
