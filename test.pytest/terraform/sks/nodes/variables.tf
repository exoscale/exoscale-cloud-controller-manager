## Tests parameters
variable "test_id" {
  description = "Unique test ID (suffix)"
  type        = string
}

variable "test_nodes_pool_size" {
  description = "Kubernetes nodes quantity"
  type        = number
  default     = 1
}


## Internal parameters
#  (shared by the control-plane)
variable "test_cluster_id" {
  description = "Kubernetes cluster ID"
  type        = string
}

variable "test_cluster_sg_id" {
  description = "Kubernetes cluster Security Group ID"
  type        = string
}

variable "test_control_plane_endpoint" {
  description = "Kubernetes cluster (API server) endpoint"
  type        = string
  default     = "(not applicable)"
}

variable "test_nodes_bootstrap_token" {
  description = "Kubernetes Nodes TLS Bootstrap token"
  type        = string
  default     = "(not applicable)"
}

variable "test_nodes_ssh_key_name" {
  description = "Nodes SSH key name"
  type        = string
  default     = "(not applicable)"
}
