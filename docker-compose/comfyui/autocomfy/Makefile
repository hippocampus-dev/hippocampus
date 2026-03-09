.DEFAULT_GOAL := all

.PHONY: install
install:
	@npm ci

.PHONY: build
build:
	@npm run build

.PHONY: all
all: install build
	@

.PHONY: dev
dev: install
	@npm run dev

.PHONY: FORCE
FORCE:
