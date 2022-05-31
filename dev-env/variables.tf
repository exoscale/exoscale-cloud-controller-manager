variable "api_key" {
  description = "Exoscale API key. If not set, will be read from environment variables."
  type        = string
  default     = ""
}

variable "api_secret" {
  description = "Exoscale API secret. If not set, will be read from environment variables."
  type        = string
  default     = ""
}

variable "environment" {
  description = "Exoscale target environment (accepted values: 'prod' or 'preprod')"
  type        = string
  default     = "prod"
}

variable "zone" {
  description = "Target zone"
  type        = string
  default     = "ch-gva-2"
}

variable "name" {
  description = "Base name of the test infrastructure"
  type        = string
  default     = "ccm-dev"
}

variable "sks_version" {
  description = "Version of Kubernetes (default is latest)"
  type        = string
  default     = null
}
