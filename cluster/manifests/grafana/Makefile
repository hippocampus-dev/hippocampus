.DEFAULT_GOAL := all

MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
CONTROLLER_DIR := $(abspath $(MAKEFILE_DIR)/../../applications/grafana-manifest-controller)
GRAFANA_TARGETS := $(shell find base/jsonnet -type f -name '*.jsonnet' | sed -r 's|base/jsonnet/|base/|' | sed -r 's|([^/]+).jsonnet|zz_generated.\1.json|')

.PHONY: link
link:
	@ln -f $(CONTROLLER_DIR)/common.libsonnet $(CONTROLLER_DIR)/jsonnetfile.json $(CONTROLLER_DIR)/jsonnetfile.lock.json .

.PHONY: install
install: link
	@docker run --mount type=bind,src=$(MAKEFILE_DIR),dst=/var/jsonnet-builder --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main --only-install

base/dashboards/kubernetes/zz_generated.%.json: base/jsonnet/dashboards/kubernetes/%.jsonnet FORCE
	@docker run --mount type=bind,src=$(MAKEFILE_DIR),dst=/var/jsonnet-builder --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main $< > $@

base/dashboards/other/zz_generated.%.json: base/jsonnet/dashboards/other/%.jsonnet FORCE
	@docker run --mount type=bind,src=$(MAKEFILE_DIR),dst=/var/jsonnet-builder --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main $< > $@

.PHONY: all
all: install $(GRAFANA_TARGETS)
	@

.PHONY: FORCE
FORCE:
