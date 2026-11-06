.DEFAULT_GOAL := all

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

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
all: fmt lint tidy test
	@

.PHONY: dev
dev:
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL go run *.go --local-address=127.0.0.1:18888 --remote-address=127.0.0.1:8888 --monitor-address=127.0.0.1:8080 --max-idle-connections 100 --min-idle-connections 10 --max-lifetime 10s --jitter-percentage 0.1

.PHONY: benchmark
benchmark:
	@cat k6/connections.js | docker compose exec -T k6 k6 --out dashboard=period=1s run -

.PHONY: FORCE
FORCE:
