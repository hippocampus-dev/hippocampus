.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@go fmt ./...
	@go install golang.org/x/tools/cmd/goimports@latest
	@$(shell go env GOPATH)/bin/goimports -w .

.PHONY: lint
lint:
	@go vet ./...

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: test
test:
	@go test -race -bench=. -benchmem -trimpath ./...

.PHONY: all
all: gen fmt lint tidy test
	@

.PHONY: gen
gen:
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
	@$(shell go env GOPATH)/bin/controller-gen paths="./..." object crd:crdVersions=v1 output:crd:artifacts:config=manifests/crd

.PHONY: dev
dev:
	@kind create cluster --name owasp-zap-controller --config kind.yaml
	@trap 'kind delete cluster --name owasp-zap-controller' EXIT ERR INT; skaffold dev --port-forward

.PHONY: FORCE
FORCE:
