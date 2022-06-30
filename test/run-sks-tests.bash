#!/bin/bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"


export TARGET_CLUSTER="sks"

source "lib/test-setup.bash"

## IMPORTANT: the order of the tests matter!
. "test-credentials-file-reload.bash"
. "test-csr-validation.bash"
# FIXME: SKS without default CCM doesn't have the "providerID: exoscale://instance-uuid" field on nodes
# so the CCM cannot update labels.
# Let's disable this test for now.
# . "test-node-labels.bash"
. "test-nlb-ingress.bash"
. "test-nlb-external.bash"
. "test-node-expunge.bash"
