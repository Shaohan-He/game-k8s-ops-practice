.PHONY: manifests test build run docker-build deploy undeploy install uninstall

IMG ?= ghcr.io/shaohan-he/game-k8s-ops-practice-operator:2.0.0

manifests:
	kubectl kustomize config/default

test:
	go test ./...

build:
	go build -o bin/manager main.go

run:
	go run ./main.go

docker-build:
	docker build -t $(IMG) .

install:
	kubectl apply -k config/crd

uninstall:
	kubectl delete -k config/crd

deploy:
	kubectl apply -k config/default

undeploy:
	kubectl delete -k config/default