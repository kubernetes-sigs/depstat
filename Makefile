.PHONY: test
test:
	go test -v ./...

.PHONY: build
build:
	go build 

.PHONY: lint
lint: 
	go get golang.org/x/lint/golint
	golint ./...
