FROM alpine:3.6

RUN apk add --no-cache ca-certificates

ADD exoscale-cloud-controller-manager /bin/

CMD ["/bin/exoscale-cloud-controller-manager"]
