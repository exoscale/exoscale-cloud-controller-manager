#!/bin/bash

set -e

nlb_assert_equal() {
    VALUE=$(exo nlb show "$EXOSCALE_LB_NAME" -z de-fra-1 --output-template "{{$1}}")
    EXPECTED="$2"
    if [ "$VALUE" != "$EXPECTED" ]
    then
        echo FAILED
        echo "error: expected value is \"$EXPECTED\", got: \"$VALUE\""
        exit 1
    fi
    echo PASS
}

nlb_service_assert_equal() {
    VALUE=$(exo nlb service show "$EXOSCALE_LB_NAME" "$EXOSCALE_LB_SERVICE_NAME" -z de-fra-1 --output-template "{{$1}}")
    EXPECTED="$2"
    if [ "$VALUE" != "$EXPECTED" ]
    then
        echo FAILED
        echo "error: expected value is \"$EXPECTED\", got: \"$VALUE\""
        exit 1
    fi
    echo PASS
}

nlb_assert_equal ".Zone" "de-fra-1"
nlb_assert_equal ".Description" ""
nlb_service_assert_equal ".Strategy" "round-robin"
nlb_service_assert_equal ".Protocol" "tcp"
nlb_service_assert_equal ".Port" "80"
nlb_service_assert_equal ".Description" ""
nlb_service_assert_equal ".Healthcheck.Mode" "tcp"
nlb_service_assert_equal ".Healthcheck.Interval" "10s"
nlb_service_assert_equal ".Healthcheck.Timeout" "5s"
nlb_service_assert_equal ".Healthcheck.Retries" "1"

EXOSCALE_LB_IP=$(kubectl get service/nginx-service -o=jsonpath="{.status.loadBalancer.ingress[*].ip}")

curl -i "http://$EXOSCALE_LB_IP"
