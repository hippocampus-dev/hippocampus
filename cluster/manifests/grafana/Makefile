.DEFAULT_GOAL := all

MAKEFILE_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
GRAFANA_TARGETS := $(shell find jsonnet -type f -name '*.jsonnet' | sed -r 's|jsonnet/||' | sed -r 's|([^/]+).jsonnet|zz_generated.\1.json|')

$(GRAFANA_TARGETS): install
	@

.PHONY: install
install:
	@docker run --mount type=bind,src=$(abspath $(MAKEFILE_DIR)),dst=/var/jsonnet-builder --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main --only-install

base/dashboards/kubernetes/zz_generated.%.json: jsonnet/base/dashboards/kubernetes/%.jsonnet FORCE
	@docker run --mount type=bind,src=$(abspath $(MAKEFILE_DIR)),dst=/var/jsonnet-builder --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main $< > $@

base/dashboards/other/zz_generated.%.json: jsonnet/base/dashboards/other/%.jsonnet FORCE
	@docker run --mount type=bind,src=$(abspath $(MAKEFILE_DIR)),dst=/var/jsonnet-builder --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main $< > $@

.PHONY: all
all: install $(GRAFANA_TARGETS)
	@

.PHONY: FORCE
FORCE:
