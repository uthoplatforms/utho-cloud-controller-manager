VERSION ?= $VERSION
REGISTRY ?= $REGISTRY

.PHONY: tidy
tidy: 
	go mod tidy
	go fmt ./...

# make new-deploy VERSION=0.1.16
.PHONY: new-deploy
new-deploy: deploy
	@sed -i 's|\(utho/utho-cloud-controller-manager:\)[0-9]*\.[0-9]*\.[0-9]*|\1$(VERSION)|g' docs/releases/latest.yml
	@kubectl apply -f docs/releases/secret.yml
	@kubectl apply -f docs/releases/latest.yml

.PHONY: deploy
deploy: tidy build push

.PHONY: build
build:
	@echo "building cloud controller with version $(VERSION)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o utho-cloud-controller-manager .
	@echo "building docker image to dockerhub utho with version $(VERSION)"
	@docker build . -t utho/utho-cloud-controller-manager:$(VERSION)

.PHONY: docker-push
push:
	@echo "building docker image to dockerhub utho with version $(VERSION)"
	docker push utho/utho-cloud-controller-manager:$(VERSION)
