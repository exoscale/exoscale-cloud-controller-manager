#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING CCM WITH EXTERNAL NLB INSTANCE"

echo "- Deploying Service"
sed -r \
  -e "s/%%EXOSCALE_ZONE%%/$EXOSCALE_ZONE/" \
  -e "s/%%EXTERNAL_NLB_ID%%/$EXTERNAL_NLB_ID/" \
  -e "s/%%NODEPOOL_ID%%/$NODEPOOL_ID/" \
  "${INTEGTEST_DIR}/manifests/hello-no-ingress.yml.tpl" \
  | kubectl $KUBECTL_OPTS apply -f -

### Test the actual NLB + service + app chain
echo "- End-to-end requests"
curl_opts="--retry 10 --retry-delay 10 --retry-connrefused --silent"
curl $curl_opts http://${EXTERNAL_NLB_IP} > /dev/null || (echo "FAIL" ; return 1)

### Test the external NLB instance properties
output_template=''
output_template+='Name={{ println .Name }}'
output_template+='Description={{ println .Description }}'

exo compute load-balancer show \
  -z $EXOSCALE_ZONE \
  --output-template "$output_template" \
  $EXTERNAL_NLB_ID > "${INTEGTEST_TMP_DIR}/external_nlb"

echo "- Checking external NLB instance properties"
while read l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  case "${k}" in
    Name) _assert_string_equal "$v" "$EXTERNAL_NLB_NAME" ;;
    Description) _assert_string_equal "$v" "$EXTERNAL_NLB_DESC" ;;
    *) echo "error: unexpected key \"$k\"" ; exit 1 ;;
  esac
done < "${INTEGTEST_TMP_DIR}/external_nlb"

echo "- Delete Service and keep external NLB instance"
sed -r \
  -e "s/%%EXOSCALE_ZONE%%/$EXOSCALE_ZONE/" \
  -e "s/%%EXTERNAL_NLB_ID%%/$EXTERNAL_NLB_ID/" \
  -e "s/%%NODEPOOL_ID%%/$NODEPOOL_ID/" \
  "${INTEGTEST_DIR}/manifests/hello-no-ingress.yml.tpl" \
  | kubectl $KUBECTL_OPTS delete -f -
_until_success "test \
  \$(exo compute load-balancer show -z \$EXOSCALE_ZONE --output-template '{{.Services|len}}' \$EXTERNAL_NLB_ID) \
  -eq 0"

echo "<<< PASS"
