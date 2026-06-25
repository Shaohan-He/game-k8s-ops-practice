.PHONY: build compose-up compose-down health k8s-up k8s-down

build:
	bash scripts/build-images.sh

compose-up:
	bash scripts/deploy-compose.sh

compose-down:
	bash scripts/clean.sh compose

health:
	bash scripts/health-check.sh

k8s-up:
	bash scripts/deploy-k8s.sh

k8s-down:
	bash scripts/clean.sh k8s

