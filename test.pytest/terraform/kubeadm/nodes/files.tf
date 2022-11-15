## Files

# Kubernetes manifests

# (applications)
resource "local_file" "app_manifest" {
  for_each = {
    "hello-external" = {
      variables = {
        exoscale_zone             = var.exoscale_zone
        exoscale_nlb_id           = exoscale_nlb.external_nlb.id
        exoscale_instance_pool_id = var.test_nodes_pool_size > 0 ? exoscale_instance_pool.nodepool[0].id : "n/a"
      }
    }
    "hello-ingress" = {}
  }

  filename = abspath("./output/manifests/${each.key}.yaml")
  content = templatefile(
    "${local.k8s_manifests_path}/${each.key}.yaml",
    try(each.value.variables, {})
  )
}
