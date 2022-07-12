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
  default     = "test-ccm-kubeadm"
}

variable "manifests" {
  description = "Manifests to deploy automatically. Manifests are expected to be present in the manifest subdirectory."
  type        = set(string)
  default     = ["cloud-controller-manager-rbac"]
}

variable "pool_size" {
  description = "Pool for CSR/expunge tests"
  type        = number
  default     = 1
}
