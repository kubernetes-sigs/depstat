.PHONY: test
test:
	go test -v ./...

.PHONY: build
build:
	go build 

.PHONY: lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
	golangci-lint run --verbose --enable gofmt
