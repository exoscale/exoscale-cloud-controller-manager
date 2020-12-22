#!/usr/bin/env bash

set -e

export INTEGTEST_DIR="${INCLUDE_PATH}/integtest"
export INTEGTEST_TMP_DIR="${INTEGTEST_DIR}/tmp"
mkdir "$INTEGTEST_TMP_DIR"

[[ -n "$EXOSCALE_API_KEY" ]]
[[ -n "$EXOSCALE_API_SECRET" ]]

source "$INTEGTEST_DIR/test-helpers.bash"

export EXOSCALE_ZONE=${EXOSCALE_ZONE:-de-fra-1}
export KUBECTL_OPTS="--timeout=600s"
export TERRAFORM_OPTS="-auto-approve -backup=-"


cleanup() {
  echo ">>> CLEANING UP"

  set +e
  terraform destroy $TERRAFORM_OPTS
  rm -rf "${INTEGTEST_TMP_DIR}"

  echo "<<< DONE"
}
trap cleanup EXIT

{
  echo ">>> DEPLOYING TEST CLUSTER INFRASTRUCTURE"

  cd "$INTEGTEST_DIR"
  printf "zone = \"%s\"\ntmpdir = \"%s\"\n" $EXOSCALE_ZONE "$INTEGTEST_TMP_DIR" > terraform.tfvars
  terraform init
  terraform apply $TERRAFORM_OPTS

  # Workaround for a problem using GitHub Action hashicorp/setup-terraform@v1:
  # https://github.com/hashicorp/setup-terraform/issues/20
  # Starting from 0.14.0 `terraform output` now displays quotes around output values:
  # https://github.com/hashicorp/terraform/issues/26831
  export TEST_ID=$(terraform-bin output -json | jq -r .test_id.value)
  export NODEPOOL_ID=$(terraform-bin output -json | jq -r .nodepool_id.value)
  export EXTERNAL_NLB_ID=$(terraform-bin output -json | jq -r .external_nlb_id.value)
  export EXTERNAL_NLB_NAME=$(terraform-bin output -json | jq -r .external_nlb_name.value)
  export EXTERNAL_NLB_DESC=$(terraform-bin output -json | jq -r .external_nlb_desc.value)
  export EXTERNAL_NLB_IP=$(terraform-bin output -json | jq -r .external_nlb_ip.value)
  export KUBECONFIG="${INTEGTEST_TMP_DIR}/kubeconfig"

  _until_success "kubectl cluster-info"

  echo "<<< DONE"
}

## IMPORTANT: the order of the tests matter!
. "${INTEGTEST_DIR}/test-nlb-ingress.bash"
. "${INTEGTEST_DIR}/test-nlb-external.bash"
. "${INTEGTEST_DIR}/test-node-labels.bash"
. "${INTEGTEST_DIR}/test-node-expunge.bash"

echo "=== ALL TESTS PASSED ==="
