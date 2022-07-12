output "external_nlb_ip" {
  value = exoscale_nlb.external.ip_address
}

output "external_nlb_id" {
  value = exoscale_nlb.external.id
}

output "node_pool_id" {
  value = var.pool_size > 0 ? exoscale_sks_nodepool.pool[0].instance_pool_id : "nop"
}
