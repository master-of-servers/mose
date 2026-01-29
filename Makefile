build:
	export GO111MODULE=on
	go install github.com/markbates/pkger/cmd/pkger@v0.17.1
	go mod download
	go build
	mkdir -p payloads

fmt:
	find . -name '*.go' -not -wholename './vendor/*' | while read -r file; do gofmt -w -s "$$file"; goimports -w "$$file"; done

lint:
	gometalinter --exclude=vendor --exclude=repos --disable-all --enable=golint --enable=vet --enable=gofmt ./...
	find . -name '*.go' | xargs gofmt -w -s

test:
	go test -count=1 -v -race ./... ; \
		echo "Testing Complete."

tidy:
	go mod tidy
	pushd cmd/puppet/main/; go mod tidy; popd
	pushd cmd/chef/main/; go mod tidy; popd
	pushd cmd/ansible/main/; go mod tidy; popd
	pushd cmd/salt/main/; go mod tidy; popd
