#!/usr/bin/env bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"
source "lib/test-helpers.bash"

###

echo ">>> TESTING CCM WITH EXTERNAL NLB INSTANCE"

echo "### Deploying Service ..."
kubectl apply -f "terraform-${TARGET_CLUSTER}/manifests/hello-no-ingress.yml" > /dev/null

### Test the actual NLB + service + app chain
EXTERNAL_NLB_ID=$(cd "terraform-${TARGET_CLUSTER}" && terraform output -raw external_nlb_id)
EXTERNAL_NLB_IP=$(cd "terraform-${TARGET_CLUSTER}" && terraform output -raw external_nlb_ip)

echo "### Checking end-to-end requests ..."
curl --retry 10 --retry-delay 10 --retry-connrefused --silent "http://${EXTERNAL_NLB_IP}" > /dev/null || (echo "!!! FAIL" >&2 ; exit 1)

echo "### Delete Service and keep external NLB instance ..."
kubectl delete -f "terraform-${TARGET_CLUSTER}/manifests/hello-no-ingress.yml" > /dev/null
_until_success "test \$(exo compute load-balancer show -z $EXOSCALE_ZONE --output-template '{{.Services|len}}' $EXTERNAL_NLB_ID) -eq 0"

echo "<<< PASS"
