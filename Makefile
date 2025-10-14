.PHONY: help format lint format-lint add-vendor test
all:
	@make help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

init:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0
	make add-vendor

##########
### GO ###
##########
format:
	golines --base-formatter="goimports" -w -m 120 .
	gofumpt -w .

lint:
	golangci-lint -c ".golangci.yml" run --allow-parallel-runners ./...

format-lint:
	make format
	make lint

add-vendor:
	go mod tidy
	go mod verify
	go mod vendor

test:
	go test -parallel=1 -count=1 ./...
