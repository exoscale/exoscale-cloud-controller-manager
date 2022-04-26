#!/usr/bin/env bash
set -e

export INTEGTEST_DIR="${INCLUDE_PATH}/integtest"
export INTEGTEST_TMP_DIR="${INTEGTEST_DIR}/tmp"

export EXOSCALE_ZONE=${EXOSCALE_ZONE:-de-fra-1}
export KUBECTL_OPTS="--timeout=600s"
export TERRAFORM_OPTS="-auto-approve -backup=-"

if [ -z "$EXOSCALE_API_KEY" ]; then
  echo 'ERROR: Missing EXOSCALE_API_KEY environment variable'
  exit
fi
if [ -z "$EXOSCALE_API_SECRET" ]; then
  echo 'ERROR: Missing EXOSCALE_API_SECRET environment variable'
  exit
fi

mkdir -p "${INTEGTEST_TMP_DIR}"
# Quirk: allow terraform destroy to work all the way through (<-> local_file resources)
touch "${INTEGTEST_TMP_DIR}/kube-ca.crt"
touch "${INTEGTEST_TMP_DIR}/cluster_endpoint"
touch "${INTEGTEST_TMP_DIR}/kubelet_join_token"

cleanup() {
  echo ">>> CLEANING UP"

  set +e
  cd "$INTEGTEST_DIR"
  if [ -e terraform.tfstate ]; then
    terraform destroy $TERRAFORM_OPTS \
      && rm -rf "${INTEGTEST_TMP_DIR}" "${INTEGTEST_DIR}/.terraform"* "${INTEGTEST_DIR}/terraform.tf"*
  fi

  echo "<<< DONE"
}
if [ "${1}" = "clean" ]; then
  cleanup
  exit
fi
# Comment next line out to keep everything "as is" (for debugging/troubleshooting purposes)
trap cleanup EXIT

## IMPORTANT: the order of the tests matter!
. "${INTEGTEST_DIR}/test-init.bash"
. "${INTEGTEST_DIR}/test-credentials-file-reload.bash"
. "${INTEGTEST_DIR}/test-csr-validation.bash"
. "${INTEGTEST_DIR}/test-node-labels.bash"
. "${INTEGTEST_DIR}/test-nlb-ingress.bash"
. "${INTEGTEST_DIR}/test-nlb-external.bash"
. "${INTEGTEST_DIR}/test-node-expunge.bash"

echo "=== ALL TESTS PASSED ==="
