.DEFAULT_GOAL := all

.PHONY: install
install:
	@npm ci

.PHONY: test
test: install
	@npm run test

.PHONY: all
all: test
	@

.PHONY: FORCE
FORCE:
