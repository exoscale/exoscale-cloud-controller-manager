## SKS parameters
variable "sks_version" {
  description = "Version of Kubernetes (default is latest)"
  type        = string
  default     = null
}


## Kubernetes parameters
variable "kubernetes_cni" {
  description = "Kubernetes CNI to use"
  type        = string
  default     = "calico"
}
