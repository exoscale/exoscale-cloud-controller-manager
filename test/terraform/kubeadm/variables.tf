## Tests parameters
variable "test_name" {
  description = "Base name of the test infrastructure"
  type        = string
  default     = "test-ccm-kubeadm"
}


## Exoscale parameters
variable "exoscale_api_key" {
  description = "Exoscale API key. If not set, will be read from environment variable (EXOSCALE_API_KEY)."
  type        = string
  default     = ""
}

variable "exoscale_api_secret" {
  description = "Exoscale API secret. If not set, will be read from environment variable (EXOSCALE_API_SECRET)."
  type        = string
  default     = ""
}

variable "exoscale_zone" {
  description = "Exoscale zone"
  type        = string
  default     = "ch-gva-2"
}

variable "exoscale_environment" {
  description = "Exoscale environment (accepted values: 'prod' or 'preprod')"
  type        = string
  default     = "prod"

  validation {
    condition     = contains(["prod", "preprod"], var.exoscale_environment)
    error_message = "'exoscale_environment' must be either 'prod' or 'preprod'"
  }
}

# Instances parameters
variable "exoscale_instance_template" {
  description = "Exoscale instance template"
  type        = string
  # TODO/FIX: control plane Pods fails under Ubuntu 22.04, when provisioning
  # them as static manifests (as Kubeadm does).
  # It's probably a bug with upstream containerd setup and conflicting updates
  # on system settings (sysctls) under Ubuntu 22.04.
  default = "Linux Ubuntu 20.04 LTS 64-bit"
}
