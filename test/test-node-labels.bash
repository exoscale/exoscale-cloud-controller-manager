#!/usr/bin/env bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"
source "lib/test-helpers.bash"

echo ">>> TESTING CCM-MANAGED KUBERNETES NODE LABELS"

echo "### Checking instance pool node labels ..."

export NODEPOOL_INSTANCE_NAME=$(kubectl get nodes -o name | sed -nE 's|^node/(test-\S*-pool-.*)$|\1|p;T next;q;:next')

ACTUAL_LABELS=$(kubectl get node ${NODEPOOL_INSTANCE_NAME} -o=go-template='{{range $k, $v := .metadata.labels}}{{$k}}={{println $v}}{{end}}')

unset EXPECTED_LABELS
declare -A EXPECTED_LABELS
EXPECTED_LABELS[kubernetes.io/hostname]="$NODEPOOL_INSTANCE_NAME"
EXPECTED_LABELS[beta.kubernetes.io/instance-type]="medium"
EXPECTED_LABELS[node.kubernetes.io/instance-type]="medium"
EXPECTED_LABELS[failure-domain.beta.kubernetes.io/region]="$EXOSCALE_ZONE"
EXPECTED_LABELS[topology.kubernetes.io/region]="$EXOSCALE_ZONE"

nodepool_labels_n=0
while IFS= read l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  # Ignore labels not managed by the Exoscale CCM
  [[ -z "${EXPECTED_LABELS[$k]}" ]] && continue

  if [[ "$v" != "${EXPECTED_LABELS[$k]}" ]]; then
    echo "!!! FAIL: Node label $k: expected \"${EXPECTED_LABELS[$k]}\", got \"$v\"" >&2
    exit 1
  fi
  (( nodepool_labels_n++ )) || true
done < <(printf '%s\n' "$ACTUAL_LABELS")
if [[ "${nodepool_labels_n}" -ne "${#EXPECTED_LABELS[*]}" ]]; then
    echo "!!! FAIL: Missing node labels: expected ${#EXPECTED_LABELS[*]}, got ${nodepool_labels_n}" >&2
    exit 1
fi


if [ "$TARGET_CLUSTER" = "sks" ]; then
  echo "### Checking external node labels: SKIPPING"
  exit 0
fi


echo "### Checking external node labels ..."

export EXTERNAL_INSTANCE_NAME=$(kubectl get nodes -o name | sed -nE 's|^node/(test-\S*-external)$|\1|p;T next;q;:next')

ACTUAL_LABELS="$(kubectl get node ${EXTERNAL_INSTANCE_NAME} -o=go-template='{{range $k, $v := .metadata.labels}}{{$k}}={{println $v}}{{end}}')"

unset EXPECTED_LABELS
declare -A EXPECTED_LABELS
EXPECTED_LABELS[kubernetes.io/hostname]="$EXTERNAL_INSTANCE_NAME"
EXPECTED_LABELS[beta.kubernetes.io/instance-type]="externalType"
EXPECTED_LABELS[node.kubernetes.io/instance-type]="externalType"
EXPECTED_LABELS[failure-domain.beta.kubernetes.io/region]="externalRegion"
EXPECTED_LABELS[topology.kubernetes.io/region]="externalRegion"

external_labels_n=0
while IFS= read l; do
  # Split "k=v" formatted line into variables $k and $v
  k=${l%=*} v=${l#*=}

  # Ignore labels not managed by the Exoscale CCM
  [[ -z "${EXPECTED_LABELS[$k]}" ]] && continue

  if [[ "$v" != "${EXPECTED_LABELS[$k]}" ]]; then
    echo "!!! FAIL: Node label $k: expected \"${EXPECTED_LABELS[$k]}\", got \"$v\"" >&2
    exit 1
  fi
  (( external_labels_n++ )) || true
done < <(printf '%s\n' "$ACTUAL_LABELS")
if [[ "${external_labels_n}" -ne "${#EXPECTED_LABELS[*]}" ]]; then
    echo "!!! FAIL: Missing node labels: expected ${#EXPECTED_LABELS[*]}, got ${external_labels_n}" >&2
    exit 1
fi

echo "<<< PASS"
