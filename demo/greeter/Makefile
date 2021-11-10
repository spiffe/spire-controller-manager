.PHONY: build
build: greeter-server greeter-client

.PHONY: greeter-server
greeter-server:
	CGO_ENABLED=0 go build ./cmd/greeter-server

.PHONY: greeter-client
greeter-client:
	CGO_ENABLED=0 go build ./cmd/greeter-client

.PHONY: docker-build
docker-build: greeter-server-image greeter-client-image

.PHONY: greeter-server-image
greeter-server-image:
	docker build --target greeter-server -t greeter-server:demo .

.PHONY: greeter-client-image
greeter-client-image:
	docker build --target greeter-client -t greeter-client:demo .

.PHONY: deploy
deploy: docker-build
	kind load docker-image greeter-server:demo --name "cluster1"
	kind load docker-image greeter-client:demo --name "cluster2"
