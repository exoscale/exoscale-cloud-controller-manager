#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING CCM-MANAGED KUBERNETES NODE LABELS"

export NODEPOOL_INSTANCE_NAME=$(kubectl get nodes \
  -o jsonpath={.items[].metadata.name} -l 'node-role.kubernetes.io/master!=')

kubectl get node $NODEPOOL_INSTANCE_NAME \
  -o=go-template='{{range $k, $v := .metadata.labels}}{{$k}}={{println $v}}{{end}}' \
  > "${INTEGTEST_TMP_DIR}/nodepool_labels"

declare -A EXPECTED
EXPECTED[kubernetes.io/hostname]="$NODEPOOL_INSTANCE_NAME"
EXPECTED[beta.kubernetes.io/instance-type]="Medium"
EXPECTED[node.kubernetes.io/instance-type]="Medium"
EXPECTED[failure-domain.beta.kubernetes.io/region]="$EXOSCALE_ZONE"
EXPECTED[topology.kubernetes.io/region]="$EXOSCALE_ZONE"

while read l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  # Ignore labels not managed by the Exoscale CCM
  [[ -z "${EXPECTED[$k]}" ]] && continue

  if [[ "$v" != "${EXPECTED[$k]}" ]]; then
    echo "FAIL: Node label $k: expected \"${EXPECTED[$k]}\", got \"$v\""
    exit 1
  fi
done < "${INTEGTEST_TMP_DIR}/nodepool_labels"

echo "<<< PASS"
