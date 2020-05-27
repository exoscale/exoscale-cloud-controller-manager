#!/bin/bash

set -e

assert_equal() {
    VALUE=$(kubectl get nodes -o=jsonpath="{.items[0].metadata.labels.$1}")
    EXPECTED="$2"
    if [ "$VALUE" != "$EXPECTED" ]
    then
        echo FAILED
        echo "error: expected value is \"$EXPECTED\", got: \"$VALUE\""
        exit 1
    fi
    echo PASS
}

assert_equal "failure-domain\.beta\.kubernetes\.io/region" "de-fra-1"
assert_equal "beta\.kubernetes\.io/instance-type" "Medium"
assert_equal "kubernetes\.io/hostname" "$EXOSCALE_INSTANCE_NAME"
assert_equal "node\.kubernetes\.io/instance-type" "Medium"
assert_equal "topology\.kubernetes\.io/region" "de-fra-1"
