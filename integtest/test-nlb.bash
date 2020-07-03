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

sleep 45

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

curl -i "http://$EXOSCALE_LB_IP"

echo "Update k8s External LoadBalancer"

envsubst < integtest/update-nlb.yaml > integtest/update-nlb.yml
kubectl apply -f "$INTEGTEST_DIR/update-nlb.yml"

sleep 45

nlb_assert_equal ".Zone" "de-fra-1"
nlb_assert_equal ".Description" "description nlb"
nlb_service_assert_equal ".Strategy" "source-hash"
nlb_service_assert_equal ".Protocol" "tcp"
nlb_service_assert_equal ".Port" "8080"
nlb_service_assert_equal ".Description" "description nlb service"
nlb_service_assert_equal ".Healthcheck.Mode" "http"
nlb_service_assert_equal ".Healthcheck.URI" "/"
nlb_service_assert_equal ".Healthcheck.Interval" "11s"
nlb_service_assert_equal ".Healthcheck.Timeout" "6s"
nlb_service_assert_equal ".Healthcheck.Retries" "2"

curl -i "http://$EXOSCALE_LB_IP:8080"