#!/bin/bash
cd "$(dirname "$(realpath -e "${0}")")" || exit
rm -v \
  api-creds* \
  *.log \
  terraform-*/.env \
  terraform-*/*.kubeconfig \
  terraform-*/*.log \
  terraform-*/*.pid \
  terraform-*/*.tfstate*

unset KUBECONFIG
unset CCM_KUBECONFIG
unset EXOSCALE_ZONE
unset EXOSCALE_SKS_AGENT_RUNNERS
unset EXOSCALE_API_CREDENTIALS_FILE

unalias approve-csr
unalias go-run-ccm
