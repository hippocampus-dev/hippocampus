.DEFAULT_GOAL := all

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))
CONTROLLER_DIRECTORY := $(abspath $(ENTRYPOINT)../../applications/grafana-manifest-controller)
GRAFANA_TARGETS := $(shell find base/jsonnet -type f -name '*.jsonnet' | sed -r 's|base/jsonnet/|base/|' | sed -r 's|([^/]+).jsonnet|zz_generated.\1.json|')
JSONNET_BUNDLER_VOLUME ?= jsonnet-bundler

.PHONY: link
link:
	@ln -f $(CONTROLLER_DIRECTORY)/common.libsonnet $(CONTROLLER_DIRECTORY)/jsonnetfile.json $(CONTROLLER_DIRECTORY)/jsonnetfile.lock.json .

.PHONY: install
install: link
	@docker run --mount type=bind,src=$(ENTRYPOINT),dst=/var/jsonnet-builder --mount type=volume,src=$(JSONNET_BUNDLER_VOLUME),dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main --only-install

base/dashboards/kubernetes/zz_generated.%.json: base/jsonnet/dashboards/kubernetes/%.jsonnet FORCE
	@docker run --mount type=bind,src=$(ENTRYPOINT),dst=/var/jsonnet-builder --mount type=volume,src=$(JSONNET_BUNDLER_VOLUME),dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main $< > $@

base/dashboards/other/zz_generated.%.json: base/jsonnet/dashboards/other/%.jsonnet FORCE
	@docker run --mount type=bind,src=$(ENTRYPOINT),dst=/var/jsonnet-builder --mount type=volume,src=$(JSONNET_BUNDLER_VOLUME),dst=/var/jsonnet-bundler ghcr.io/hippocampus-dev/hippocampus/jsonnet-builder:main $< > $@

.PHONY: all
all: install $(GRAFANA_TARGETS)
	@

.PHONY: FORCE
FORCE:
