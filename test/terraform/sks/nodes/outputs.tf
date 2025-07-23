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
  value = var.test_cluster_id
}

# Nodes

# (pool)
output "nodepool_id" {
  value = var.test_nodes_pool_size > 0 ? exoscale_sks_nodepool.nodepool[0].id : "n/a"
}
output "instancepool_id" {
  value = var.test_nodes_pool_size > 0 ? exoscale_sks_nodepool.nodepool[0].instance_pool_id : "n/a"
}

# (external)
output "external_node_name" {
  value = "n/a"
}
output "external_node_ipv4" {
  value = "n/a"
}
output "external_node_ipv6" {
  value = "n/a"
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
output "manifest_udp_echo" {
  value = local_file.app_manifest["udp-echo"].filename
}
