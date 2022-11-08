#!/bin/bash

if [ -z "$EXOSCALE_API_KEY" ]; then
  echo '!!! ERROR: Missing EXOSCALE_API_KEY environment variable'
  exit
fi
if [ -z "$EXOSCALE_API_SECRET" ]; then
  echo '!!! ERROR: Missing EXOSCALE_API_SECRET environment variable'
  exit
fi

terraform_create(){
  echo "### Creating TEST environment ($TARGET_CLUSTER)"

  cd "terraform-${TARGET_CLUSTER}" || exit
  echo "### Initializing Terraform (plugins and providers)"
  echo "  # (see 'terraform-${TARGET_CLUSTER}/terraform-init.log')"
  terraform init -upgrade > terraform-init.log
  echo "### Terraforming the test infrastructure"
  echo "  # (see 'terraform-${TARGET_CLUSTER}/terraform-apply.log')"
  terraform apply -auto-approve > terraform-create.log
  cd - > /dev/null || exit
}

terraform_destroy() {
  echo "### Tearing down TEST environment ($TARGET_CLUSTER)"

  cd "terraform-${TARGET_CLUSTER}" || exit
  echo "### Destroying Terraformed test infrastructure"
  echo "  # (see 'terraform-${TARGET_CLUSTER}/terraform-destroy.log')"
  terraform destroy -auto-approve > terraform-destroy.log
  cd - > /dev/null || exit
}

clean() {
  ./ccm-kill
  terraform_destroy
}

trap clean EXIT

terraform_create

. "terraform-${TARGET_CLUSTER}/.env"
./ccm-start
