FROM golang:1.14.2-alpine as builder

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

WORKDIR /go/src/github.com/exoscale/exoscale-cloud-controller-manager

COPY . .

ARG TAG
RUN CGO_ENABLED=0 \
    go build -mod vendor \
    -ldflags "-w -s -X github.com/exoscale/exoscale-cloud-controller-manager/exoscale.version=${TAG}" \
    -o ./bin/exoscale-cloud-controller-manager \
    ./cmd/exoscale-cloud-controller-manager

FROM scratch
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/src/github.com/exoscale/exoscale-cloud-controller-manager/bin/exoscale-cloud-controller-manager .
ENTRYPOINT ["/exoscale-cloud-controller-manager"]