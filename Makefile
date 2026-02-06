DEPSTAT_BIN ?= ./bin/depstat
K8S_DIR ?= ../k8s.io/kubernetes
ARTIFACT_DIR ?= ./_artifacts/kubernetes-smoke

.PHONY: help
help:
	@echo "Common targets:"
	@echo "  make build               Build depstat binary"
	@echo "  make test                Run unit tests"
	@echo "  make lint                Run golangci-lint"
	@echo "  make ci-fixture          Run deterministic CLI integration fixture"
	@echo "  make ci-kubernetes-smoke Run CLI smoke tests against Kubernetes checkout"
	@echo "  make clean               Remove build and artifact directories"
	@echo ""
	@echo "Override vars as needed:"
	@echo "  DEPSTAT_BIN=$(DEPSTAT_BIN)"
	@echo "  K8S_DIR=$(K8S_DIR)"
	@echo "  ARTIFACT_DIR=$(ARTIFACT_DIR)"

.PHONY: build
build:
	mkdir -p ./bin
	go build -o $(DEPSTAT_BIN) .

.PHONY: test
test:
	go test -v ./...

.PHONY: lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
	golangci-lint run --verbose --enable gofmt

.PHONY: ci-fixture
ci-fixture: build
	bash ./hack/ci/fixture-integration.sh "$(DEPSTAT_BIN)"

.PHONY: ci-kubernetes-smoke
ci-kubernetes-smoke: build
	@test -d "$(K8S_DIR)" || { echo "K8S_DIR=$(K8S_DIR) not found"; exit 1; }
	bash ./hack/ci/kubernetes-smoke.sh "$(DEPSTAT_BIN)" "$(K8S_DIR)" "$(ARTIFACT_DIR)"

.PHONY: clean
clean:
	rm -rf ./bin ./_artifacts
