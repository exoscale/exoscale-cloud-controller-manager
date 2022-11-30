## Providers
terraform {
  required_providers {
    # Exoscale
    exoscale = {
      source  = "exoscale/exoscale"
      version = ">=0.38.0"
    }
  }
}

# Exoscale
# REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs
provider "exoscale" {
  key         = var.exoscale_api_key != "" ? var.exoscale_api_key : null
  secret      = var.exoscale_api_secret != "" ? var.exoscale_api_secret : null
  environment = var.exoscale_environment == "preprod" ? "ppapi" : null
  timeout     = 600
}
