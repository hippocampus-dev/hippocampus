.DEFAULT_GOAL := all

.PHONY: all
all:
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0
	@$(shell go env GOPATH)/bin/controller-gen paths="./..." object crd:crdVersions=v1 output:crd:artifacts:config=manifests/crd

.PHONY: dev
dev:
	@skaffold dev --port-forward
