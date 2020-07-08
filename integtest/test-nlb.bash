#!/usr/bin/env bash

set -e
set -u
# set -x

nlb_service_assert_equal() {
    VALUE=$(exo nlb service show "$EXOSCALE_LB_NAME" "$1" --zone de-fra-1 --output-template "{{$2}}")
    EXPECTED="$3"
    if [ "$VALUE" != "$EXPECTED" ]
    then
        echo FAILED
        echo "error: expected value is \"$EXPECTED\", got: \"$VALUE\""
        exit 1
    fi
    echo PASS
}

nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Strategy" "round-robin"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Protocol" "tcp"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Port" "80"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Description" ""
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Healthcheck.Mode" "tcp"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Healthcheck.Interval" "10s"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Healthcheck.Timeout" "5s"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME1" ".Healthcheck.Retries" "1"

EXOSCALE_LB_IP=$(kubectl get service/nginx-service -o=jsonpath="{.status.loadBalancer.ingress[*].ip}")

curl -i --silent --output /dev/null "http://$EXOSCALE_LB_IP"

kubectl delete service/nginx-service --timeout=180s

nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Strategy" "source-hash"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Protocol" "tcp"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Port" "8080"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Description" "description-nlb-service"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Healthcheck.Mode" "http"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Healthcheck.URI" "/"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Healthcheck.Interval" "11s"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Healthcheck.Timeout" "6s"
nlb_service_assert_equal "$EXOSCALE_LB_SERVICE_NAME2" ".Healthcheck.Retries" "2"

curl -i --silent --output /dev/null "http://$EXOSCALE_LB_IP:8080"

kubectl delete service/nginx-service-2 --timeout=180s
