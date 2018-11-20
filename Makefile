TAG?=latest
.PHONY: build

build:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t pfelipefeitosa/faas-swarm:$(TAG) .

test-unit:
	go test -v $(go list ./... | grep -v /vendor/) -cover

build-armhf:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t pfelipefeitosa/faas-swarm:$(TAG)-armhf . -f Dockerfile.armhf

push:
	docker push pfelipefeitosa/faas-swarm:$(TAG)

all: build

ci-armhf-build:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t pfelipefeitosa/faas-swarm:$(TAG)-armhf . -f Dockerfile.armhf

ci-armhf-push:
	docker push pfelipefeitosa/faas-swarm:$(TAG)-armhf

