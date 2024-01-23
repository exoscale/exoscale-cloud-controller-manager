## Outputs

# Test
output "test_id" {
  value = random_string.test_id.result
}
output "test_name" {
  value = local.test_name
}

# Exoscale
output "exoscale_zone" {
  value = var.exoscale_zone
}
output "exoscale_environment" {
  value = var.exoscale_environment
}

# Cluster
output "cluster_id" {
  value = random_string.test_id.result
}
output "cluster_sg_id" {
  value = exoscale_security_group.cluster_sg.id
}

# Control plane
output "control_plane_node" {
  value = exoscale_compute_instance.control_plane.name
}
output "control_plane_ipv4" {
  value = exoscale_compute_instance.control_plane.public_ip_address
}
output "control_plane_endpoint" {
  value = "https://${exoscale_compute_instance.control_plane.public_ip_address}:6443"
}

# Nodes
output "nodes_bootstrap_token" {
  value = "${random_string.bootstrap_token_id.result}.${random_string.bootstrap_token_secret.result}"
}
output "nodes_ssh_key_name" {
  value = exoscale_ssh_key.ssh_key.name
}
output "nodes_ssh_key" {
  value = local_sensitive_file.ssh_key.filename
}
output "nodes_ssh_username" {
  value = data.exoscale_template.node_template.default_user
}

# Kubernetes configuration and credentials
output "kubernetes_cni" {
  value = var.kubernetes_cni
}
output "kubeconfig_admin" {
  value = local_sensitive_file.kubeconfig["admin"].filename
}
output "kubeconfig_ccm" {
  value = local_sensitive_file.kubeconfig["ccm"].filename
}

# CCM configuration and credentials
output "ccm_rbac" {
  value = local_file.ccm_rbac.filename
}
output "ccm_cloud_config" {
  value = local_file.ccm_cloud_config.filename
}
output "ccm_api_credentials" {
  value = local.ccm_api_credentials_path
}

# (source and executable)
output "ccm_main" {
  value = local.ccm_main_path
}
output "ccm_exe" {
  value = local.ccm_exe_path
}

# Development/troubleshooting shell environment
output "shell_environment" {
  value = local_file.shell_environment.filename
}
