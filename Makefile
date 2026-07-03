ifeq ($(OS),Windows_NT)
VALIDATE_CMD := powershell -ExecutionPolicy Bypass -File scripts/validate.ps1
else
VALIDATE_CMD := bash scripts/validate.sh
endif

.PHONY: build compose-up compose-down health validate k8s-up k8s-down operator-manifests operator-build operator-run operator-deploy operator-undeploy k8s-v2-up

build:
	bash scripts/build-images.sh

validate:
	$(VALIDATE_CMD)

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

operator-manifests:
	$(MAKE) -C operator manifests

operator-build:
	$(MAKE) -C operator build

operator-run:
	$(MAKE) -C operator run

operator-deploy:
	$(MAKE) -C operator deploy

operator-undeploy:
	$(MAKE) -C operator undeploy

k8s-v2-up:
	kubectl apply -k k8s-v2