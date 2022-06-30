#!/bin/bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"


export TARGET_CLUSTER="sks"

source "lib/test-setup.bash"

## IMPORTANT: the order of the tests matter!
. "test-credentials-file-reload.bash"
. "test-csr-validation.bash"
. "test-node-labels.bash"
. "test-nlb-ingress.bash"
. "test-nlb-external.bash"
. "test-node-expunge.bash"
