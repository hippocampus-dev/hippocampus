.DEFAULT_GOAL := all

TARGETS := $(wildcard ../pages/*.html)

.PHONY: all
all: $(TARGETS)
	@

../pages/%.html: %.md template.html
	@pandoc \
		--standalone \
		--template=template.html \
		--metadata title="Resume" \
		-o $@ \
		$<
