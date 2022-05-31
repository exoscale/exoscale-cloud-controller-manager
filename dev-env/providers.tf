provider "exoscale" {
  key         = var.api_key != "" ? var.api_key : null
  secret      = var.api_secret != "" ? var.api_secret : null
  environment = var.environment == "preprod" ? "ppapi" : null
  timeout     = 600
}