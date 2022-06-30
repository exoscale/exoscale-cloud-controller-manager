output "external_nlb_ip" {
  value = exoscale_nlb.external.ip_address
}

output "external_nlb_id" {
  value = exoscale_nlb.external.id
}

output "node_pool_id" {
  value = var.pool_size > 0 ? exoscale_instance_pool.test[0].id : "nop"
}