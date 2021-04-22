.PHONY: test
test:
	go test -v ./...

.PHONY: build
build:
	go build 

.PHONY: lint
lint: 
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sudo sh -s -- -b $(GOPATH_DIR)/bin v1.30.0
	@$(GOPATH_DIR)/bin/golangci-lint run -v
