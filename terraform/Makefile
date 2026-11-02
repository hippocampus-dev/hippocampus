.DEFAULT_GOAL := plan

.PHONY: init
init:
	@dotenvx run -- terraform init

.PHONY: plan
plan: init
	@dotenvx run -- terraform plan

.PHONY: apply
apply:
	@dotenvx run -- terraform apply

.PHONY: output
output:
	@dotenvx run -- terraform output

.PHONY: FORCE
FORCE:
