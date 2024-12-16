FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY utho-cloud-controller-manager /usr/local/bin/utho-cloud-controller-manager

ENTRYPOINT ["utho-cloud-controller-manager"] 
