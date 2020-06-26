build:
	export GO111MODULE=on
	go get
	go build

setup-linter: ## Install all the build and lint dependencies
	gometalinter --install

fmt: ## gofmt and goimports all go files
	find . -name '*.go' -not -wholename './vendor/*' | while read -r file; do gofmt -w -s "$$file"; goimports -w "$$file"; done

lint: ## Run all the linters
	golangci-lint run \
		--no-config \
		--issues-exit-code=0 \
		--timeout=30m \
		--disable-all \
		--enable=deadcode \
		--enable=gocyclo \
		--enable=golint \
		--enable=varcheck \
		--enable=structcheck \
		--enable=maligned \
		--enable=errcheck \
		--enable=dupl \
		--enable=ineffassign \
		--enable=interfacer \
		--enable=unconvert \
		--enable=goconst \
		--enable=gosec \
		--enable=megacheck 
	markdownfmt -w README.md

test:
	go test -count=1 -v -race ./... ; \
		echo "Testing Complete."
