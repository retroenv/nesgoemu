GOLANGCI_VERSION = v1.55.2

help: ## show help, shown by default if no target is specified
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

lint: ## run code linters
	golangci-lint run

build-all: ## build code
	go build ./...
	go build -tags noopengl,sdl ./...
	go build -tags nogui ./...

test: install ## run tests
	go test -timeout 10s -race ./...

test-coverage: ## run unit tests and create test coverage
	go test -timeout 10s ./... -coverprofile .testCoverage -covermode=atomic -coverpkg=./...

test-coverage-web: test-coverage ## run unit tests and show test coverage in browser
	go tool cover -func .testCoverage | grep total | awk '{print "Total coverage: "$$3}'
	go tool cover -html=.testCoverage

install: ## install all binaries
	go install -buildvcs=false .

install-linters: ## install all used linters
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_VERSION}

release: ## build release binaries for current git tag and publish on github
	goreleaser release

release-snapshot: ## build release binaries from current git state as snapshot
	goreleaser release --snapshot --clean

test-no-gui: install-no-gui run-tests ## run unit tests with gui disabled
	go test -timeout 10s -tags nogui ./...

install-no-gui: ## install all binaries with gui disabled
	go install -tags nogui ./cmd/...
