FROM golang:1.23-alpine AS build

RUN apk add --no-cache git

WORKDIR /workspace

COPY . .
ARG VERSION

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o utho-cloud-controller-manager .

FROM alpine:latest
RUN apk add --no-cache ca-certificates

COPY --from=build /workspace/utho-cloud-controller-manager /usr/local/bin/utho-cloud-controller-manager
ENTRYPOINT ["utho-cloud-controller-manager"] 
