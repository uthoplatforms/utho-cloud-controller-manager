VERSION ?= $VERSION
REGISTRY ?= $REGISTRY

.PHONY: tidy
tidy: 
	go mod tidy
	go fmt ./...

.PHONY: deploy
deploy: clean docker-build docker-push

.PHONY: build
build: tidy
	@echo "building utho ccm"
	go build -trimpath -o utho-cloud-controller-manager .

.PHONY: build-linux
build-linux:
	@echo "building utho ccm for linux"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/utho-cloud-controller-manager .

.PHONY: docker-build
docker-build:
	@echo "building docker image to dockerhub $(REGISTRY) with version $(VERSION)"
	docker build . -t $(REGISTRY)/utho-cloud-controller-manager:$(VERSION)

.PHONY: docker-push
docker-push:
	docker push $(REGISTRY)/utho-cloud-controller-manager:$(VERSION)

.PHONY: clean
clean: tidy
	go clean -i -x ./...

.PHONY: test
test:
	go test -race github.com/utho/utho-cloud-controller-manager/utho -v