#!/usr/bin/env bash

set -e
set -u
# set -x

k8s_assert_equal() {
    VALUE=$(kubectl get node "$1" -o=jsonpath="{.metadata.labels.$2}")
    EXPECTED="$3"
    if [ "$VALUE" != "$EXPECTED" ]
    then
        echo FAILED
        echo "error: $1: expected value is \"$EXPECTED\", got: \"$VALUE\""
        exit 1
    fi
    echo PASS
}

k8s_assert_equal "$EXOSCALE_MASTER_NAME" "failure-domain\.beta\.kubernetes\.io/region" "de-fra-1"
k8s_assert_equal "$EXOSCALE_MASTER_NAME" "beta\.kubernetes\.io/instance-type" "Medium"
k8s_assert_equal "$EXOSCALE_MASTER_NAME" "kubernetes\.io/hostname" "$EXOSCALE_MASTER_NAME"
k8s_assert_equal "$EXOSCALE_MASTER_NAME" "node\.kubernetes\.io/instance-type" "Medium"
k8s_assert_equal "$EXOSCALE_MASTER_NAME" "topology\.kubernetes\.io/region" "de-fra-1"

k8s_assert_equal "$EXOSCALE_NODE_NAME" "failure-domain\.beta\.kubernetes\.io/region" "de-fra-1"
k8s_assert_equal "$EXOSCALE_NODE_NAME" "beta\.kubernetes\.io/instance-type" "Medium"
k8s_assert_equal "$EXOSCALE_NODE_NAME" "kubernetes\.io/hostname" "$EXOSCALE_NODE_NAME"
k8s_assert_equal "$EXOSCALE_NODE_NAME" "node\.kubernetes\.io/instance-type" "Medium"
k8s_assert_equal "$EXOSCALE_NODE_NAME" "topology\.kubernetes\.io/region" "de-fra-1"
