## Outputs

# Test
output "test_id" {
  value = var.test_id
}
output "test_name" {
  value = local.test_name
}
output "test_nodes_pool_size" {
  value = var.test_nodes_pool_size
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
  value = var.test_id
}

# Nodes

# (pool)
output "nodepool_id" {
  value = "n/a"
}
output "instancepool_id" {
  value = var.test_nodes_pool_size > 0 ? exoscale_instance_pool.nodepool[0].id : "n/a"
}

# (external)
output "external_node_name" {
  value = exoscale_compute_instance.external_node.name
}
output "external_node_ipv4" {
  value = exoscale_compute_instance.external_node.public_ip_address
}
output "external_node_ipv6" {
  value = exoscale_compute_instance.external_node.ipv6_address
}
output "external_node_kubeconfig" {
  value = local_sensitive_file.kubeconfig["external-node"].filename
}

# Load balancer (NLB)
output "external_nlb_id" {
  value = exoscale_nlb.external_nlb.id
}
output "external_nlb_ipv4" {
  value = exoscale_nlb.external_nlb.ip_address
}

# Kubernetes manifests
output "manifest_hello_external" {
  value = local_file.app_manifest["hello-external"].filename
}
output "manifest_hello_ingress" {
  value = local_file.app_manifest["hello-ingress"].filename
}
