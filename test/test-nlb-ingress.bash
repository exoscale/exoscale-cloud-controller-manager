#!/usr/bin/env bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"
source "lib/test-helpers.bash"

###

echo ">>> TESTING CCM-MANAGED NLB INSTANCE"

echo "### Deploying cluster ingress controller ..."
kubectl apply -f "manifests/ingress-nginx.namespace.yml" > /dev/null
kubectl apply -f "manifests/ingress-nginx.yml" > /dev/null
kubectl wait --timeout 600s --for condition=Available --namespace ingress-nginx deployment.apps/ingress-nginx-controller > /dev/null
# It is not possible to `kubectl wait` on an Ingress resource, so we wait until
# we see a public IP address associated to the Service Load Balancer...
_until_success "test -n \"\$(kubectl --namespace ingress-nginx get svc/ingress-nginx-controller -o=jsonpath='{.status.loadBalancer.ingress[].ip}')\""

export INGRESS_NLB_IP=$(kubectl --namespace ingress-nginx get svc/ingress-nginx-controller -o=jsonpath='{.status.loadBalancer.ingress[].ip}')
export INGRESS_NLB_ID=$(exo compute load-balancer list -z "$EXOSCALE_ZONE" -O text | awk "/${INGRESS_NLB_IP}/ { print \$1 }")
export NODEPOOL_ID=$(cd "terraform-${TARGET_CLUSTER}" && terraform output -raw node_pool_id)

echo "### Deploying test application ..."
kubectl apply -f "manifests/hello-ingress.yml" > /dev/null
kubectl wait --timeout 600s --for condition=Available deployment.apps/hello > /dev/null

### Test the actual NLB + ingress-nginx controller + service + app chain
echo "### Checking end-to-end requests ..."

curl --retry 10 --retry-delay 5 --retry-connrefused -s "http://$INGRESS_NLB_IP" > /dev/null || (echo "!!! FAIL" >&2 && false)
curl --retry 10 --retry-delay 5 --retry-connrefused -sk "https://$INGRESS_NLB_IP" > /dev/null || (echo "!!! FAIL" >&2 && false)


### Test the generated NLB services' properties
output_template=''
output_template+='Name={{ println .Name }}'
output_template+='InstancePoolID={{ println .InstancePoolID }}'
output_template+='Protocol={{ println .Protocol }}'
output_template+='Port={{ println .Port }}'
output_template+='Strategy={{ println .Strategy }}'
output_template+='HealthcheckMode={{ println .Healthcheck.Mode }}'
output_template+='HealthcheckInterval={{ println .Healthcheck.Interval }}'
output_template+='HealthcheckTimeout={{ println .Healthcheck.Timeout }}'
output_template+='HealthcheckRetries={{ println .Healthcheck.Retries }}'

exo compute load-balancer show \
  --output-template '{{range .Services}}{{println .ID}}{{end}}' \
  -z "$EXOSCALE_ZONE" "$INGRESS_NLB_ID" | while read -r svcid; do
    exo compute load-balancer service show \
      -z "$EXOSCALE_ZONE" \
      --output-template "$output_template" \
      "$INGRESS_NLB_ID" "$svcid" > "terraform-$TARGET_CLUSTER/nlb_service_${svcid}"

    svcport=$(awk -F= '$1 == "Port" {print $2}' < "terraform-$TARGET_CLUSTER/nlb_service_${svcid}")
    case $svcport in
    80)
      mv "terraform-$TARGET_CLUSTER/nlb_service_${svcid}" "terraform-$TARGET_CLUSTER/nlb_service_http"
      export INGRESS_NLB_SERVICE_HTTP_ID=$svcid
      ;;
    443)
      mv "terraform-$TARGET_CLUSTER/nlb_service_${svcid}" "terraform-$TARGET_CLUSTER/nlb_service_https"
      export INGRESS_NLB_SERVICE_HTTPS_ID=$svcid
      ;;
    *)
      echo "!!! ERROR: unexpected service port $svcport, expected either 80 or 443" >&2
      exit 1
      ;;
    esac
done

## HTTP service
echo "### Checking ingress HTTP NLB service properties ..."
while read -r l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  case "${k}" in
    Name) _assert_string_match "$v" "-80$" ;;
    InstancePoolID) _assert_string_equal "$v" "$NODEPOOL_ID" ;;
    Protocol) _assert_string_equal "$v" "tcp" ;;
    Port) _assert_string_equal "$v" "80" ;;
    Strategy) _assert_string_equal "$v" "round-robin" ;;
    HealthcheckMode) _assert_string_equal "$v" "tcp" ;;
    HealthcheckInterval) _assert_string_equal "$v" "10s" ;;
    HealthcheckTimeout) _assert_string_equal "$v" "5s" ;;
    HealthcheckRetries) _assert_string_equal "$v" "1" ;;
    *) echo "!!! ERROR: unexpected key \"$k\"" >&2 ; exit 1 ;;
  esac
done < "terraform-$TARGET_CLUSTER/nlb_service_http"

## HTTPS service
echo "### Checking ingress HTTPS NLB service properties ..."
while read -r l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  case "${k}" in
    Name) _assert_string_match "$v" "-443$" ;;
    InstancePoolID) _assert_string_equal "$v" "$NODEPOOL_ID" ;;
    Protocol) _assert_string_equal "$v" "tcp" ;;
    Port) _assert_string_equal "$v" "443" ;;
    Strategy) _assert_string_equal "$v" "round-robin" ;;
    HealthcheckMode) _assert_string_equal "$v" "tcp" ;;
    HealthcheckInterval) _assert_string_equal "$v" "10s" ;;
    HealthcheckTimeout) _assert_string_equal "$v" "5s" ;;
    HealthcheckRetries) _assert_string_equal "$v" "1" ;;
    *) echo "!!! ERROR: unexpected key \"$k\"" >&2 ; exit 1 ;;
  esac
done < "terraform-$TARGET_CLUSTER/nlb_service_https"

## Updating ingress controller Service to switch NLB service health checking to "http" mode
echo "### Updating ingress NLB services ..."
patch='{"metadata":{"annotations":{'
patch+='"service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-mode":"http",'
patch+='"service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-uri":"/"'
patch+='}}}'
kubectl -n ingress-nginx patch svc ingress-nginx-controller -p "$patch" > /dev/null
_until_success "test \"\$(exo compute load-balancer show --output-template '{{range .Services}}{{println .ID}}{{end}}' -z $EXOSCALE_ZONE $INGRESS_NLB_ID | while read svcid; do
    exo compute load-balancer service show -z $EXOSCALE_ZONE --output-template '{{.Healthcheck.Mode}}' $INGRESS_NLB_ID \$svcid ; done)\" == \"httphttp\""

echo "### Destroying test application ..."
kubectl delete -f "manifests/hello-ingress.yml" > /dev/null

## Before handing out to the cleanup phase, delete the ingress controller Service in order
## to delete the managed NLB instance, otherwise it won't be possible to delete the
## cluster Nodepool's Instance Pool.
echo "### Deleting ingress NLB ..."
kubectl delete -f "manifests/ingress-nginx.yml" > /dev/null
_until_success "test ! \$(exo compute load-balancer show -z $EXOSCALE_ZONE} $INGRESS_NLB_ID 2>/dev/null)"
kubectl delete -f "manifests/ingress-nginx.namespace.yml" > /dev/null

echo "<<< PASS"
