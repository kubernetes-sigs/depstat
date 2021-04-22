.PHONY: test
test:
	go test -v ./...

.PHONY: build
build:
	go build 

.PHONY: lint
lint: 
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.39.0
	./bin/golangci-lint run -v
