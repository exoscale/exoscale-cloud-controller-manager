FROM golang:1.17-alpine as builder

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

WORKDIR /go/src/github.com/exoscale/exoscale-cloud-controller-manager

COPY . .

ARG VERSION
RUN CGO_ENABLED=0 \
    go build -mod vendor \
    -ldflags "-w -s -X github.com/exoscale/exoscale-cloud-controller-manager/exoscale.version=${VERSION}" \
    -o ./bin/exoscale-cloud-controller-manager \
    ./cmd/exoscale-cloud-controller-manager

FROM busybox:1.32.0

ARG VERSION
ARG VCS_REF
ARG BUILD_DATE

LABEL org.label-schema.build-date=${BUILD_DATE} \
      org.label-schema.vcs-ref=${VCS_REF} \
      org.label-schema.vcs-url="https://github.com/exoscale/exoscale-cloud-controller-manager" \
      org.label-schema.version=${VERSION} \
      org.label-schema.name="exoscale-cloud-controller-manager" \
      org.label-schema.vendor="Exoscale" \
      org.label-schema.description="Exoscale Cloud Controller Manager" \
      org.label-schema.schema-version="1.0"


WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/src/github.com/exoscale/exoscale-cloud-controller-manager/bin/exoscale-cloud-controller-manager .
ENTRYPOINT ["/exoscale-cloud-controller-manager"]
