.DEFAULT_GOAL := dev

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))
MODEL_DIRECTORY ?= $(HOME)/.cache/meegeren
TORCH_BACKEND ?= cu130
PIPELINE ?= quality-fast
UPSCALER ?= real-esrgan

.PHONY: fmt
fmt:
	@uvx ruff format

.PHONY: lint
lint:
	@uvx ruff check --fix

.PHONY: all
all: fmt lint
	@

.PHONY: e2e
e2e:
	@./e2e.sh $(PIPELINES)

.PHONY: install
install:
	@uv sync --frozen --extra $(TORCH_BACKEND)

# Rootless Docker maps the container's root, not its nonroot user, to the invoking user who owns MODEL_DIRECTORY
.PHONY: download
download:
	@docker build -t meegeren-downloader $(ENTRYPOINT)downloader
	@mkdir -p $(MODEL_DIRECTORY)
	@docker run --rm --user 0:0 --mount type=bind,src=$(MODEL_DIRECTORY),dst=/home/nonroot meegeren-downloader -i /var/aria2c/links.txt
	@docker run --rm --user 0:0 --mount type=bind,src=$(MODEL_DIRECTORY),dst=/home/nonroot meegeren-downloader -i /var/aria2c/gated-links.txt --header "Authorization: Bearer $(HF_HUB_TOKEN)"

.PHONY: dev
dev: install
	@PIPELINE=$(PIPELINE) UPSCALER=$(UPSCALER) PYTORCH_ALLOC_CONF=pinned_max_round_threshold_mb:0 watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL uv run --extra $(TORCH_BACKEND) -- python main.py

.PHONY: FORCE
FORCE:
