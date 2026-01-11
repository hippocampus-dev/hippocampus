.DEFAULT_GOAL := openapi

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: openapi
openapi:
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	@mkdir -p openapi
	@PATH="$(PATH):$(shell go env GOPATH)/bin" protoc \
		--proto_path=../../proto \
		--proto_path=../../proto/googleapis \
		--openapiv2_out=openapi \
		--openapiv2_opt=allow_merge=true,merge_file_name=api \
		--openapiv2_opt=Mhelloworld.proto=github.com/hippocampus-dev/hippocampus/proto/helloworld \
		--openapiv2_opt=Mhippocampus.proto=github.com/hippocampus-dev/hippocampus/proto/hippocampus \
		../../proto/helloworld.proto ../../proto/hippocampus.proto

.PHONY: dev
dev:
	@watchexec -Nc -rw $(ENTRYPOINT) --stop-signal SIGKILL mold --run cargo run -- --address 127.0.0.1:8080 --monitor-address 127.0.0.1:8081

.PHONY: FORCE
FORCE:
