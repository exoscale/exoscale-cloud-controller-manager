#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING CCM-MANAGED KUBERNETES NODE LABELS"

echo "### Checking instance pool node labels ..."

export NODEPOOL_INSTANCE_NAME=$(kubectl get nodes -o name | sed -nE 's|^node/(test-\S*-pool-.*)$|\1|p;T next;q;:next')

kubectl get node ${NODEPOOL_INSTANCE_NAME} \
  -o=go-template='{{range $k, $v := .metadata.labels}}{{$k}}={{println $v}}{{end}}' \
  > "${INTEGTEST_TMP_DIR}/nodepool_labels"

unset EXPECTED
declare -A EXPECTED
EXPECTED[kubernetes.io/hostname]="$NODEPOOL_INSTANCE_NAME"
EXPECTED[beta.kubernetes.io/instance-type]="medium"
EXPECTED[node.kubernetes.io/instance-type]="medium"
EXPECTED[failure-domain.beta.kubernetes.io/region]="$EXOSCALE_ZONE"
EXPECTED[topology.kubernetes.io/region]="$EXOSCALE_ZONE"

nodepool_labels_n=0
while read l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  # Ignore labels not managed by the Exoscale CCM
  [[ -z "${EXPECTED[$k]}" ]] && continue

  if [[ "$v" != "${EXPECTED[$k]}" ]]; then
    echo "FAIL: Node label $k: expected \"${EXPECTED[$k]}\", got \"$v\""
    exit 1
  fi
  (( nodepool_labels_n++ )) || true
done < "${INTEGTEST_TMP_DIR}/nodepool_labels"
if [[ "${nodepool_labels_n}" -ne "${#EXPECTED[*]}" ]]; then
    echo "FAIL: Missing node labels: expected ${#EXPECTED[*]}, got ${nodepool_labels_n}"
    exit 1
fi

echo "### Checking external node labels ..."

export EXTERNAL_INSTANCE_NAME=$(kubectl get nodes -o name | sed -nE 's|^node/(test-\S*-external)$|\1|p;T next;q;:next')

kubectl get node ${EXTERNAL_INSTANCE_NAME} \
  -o=go-template='{{range $k, $v := .metadata.labels}}{{$k}}={{println $v}}{{end}}' \
  > "${INTEGTEST_TMP_DIR}/external_labels"

unset EXPECTED
declare -A EXPECTED
EXPECTED[kubernetes.io/hostname]="$EXTERNAL_INSTANCE_NAME"
EXPECTED[beta.kubernetes.io/instance-type]="externalType"
EXPECTED[node.kubernetes.io/instance-type]="externalType"
EXPECTED[failure-domain.beta.kubernetes.io/region]="externalRegion"
EXPECTED[topology.kubernetes.io/region]="externalRegion"

external_labels_n=0
while read l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  # Ignore labels not managed by the Exoscale CCM
  [[ -z "${EXPECTED[$k]}" ]] && continue

  if [[ "$v" != "${EXPECTED[$k]}" ]]; then
    echo "FAIL: Node label $k: expected \"${EXPECTED[$k]}\", got \"$v\""
    exit 1
  fi
  (( external_labels_n++ )) || true
done < "${INTEGTEST_TMP_DIR}/external_labels"
if [[ "${external_labels_n}" -ne "${#EXPECTED[*]}" ]]; then
    echo "FAIL: Missing node labels: expected ${#EXPECTED[*]}, got ${external_labels_n}"
    exit 1
fi

echo "<<< PASS"
