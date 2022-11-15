## Kubernetes parameters
variable "kubernetes_cni" {
  description = "Kubernetes CNI to use"
  type        = string
  default     = "calico"
}
