## Files

# Configuration

# (Kubeconfig)
resource "local_sensitive_file" "kubeconfig" {
  for_each = {
    "external-node" = { user = "system:node:${exoscale_compute_instance.external_node.name}" }
  }

  filename        = abspath("${path.module}/output/${each.key}.kubeconfig")
  file_permission = "0600"
  content = templatefile(
    "${path.module}/resources/kubeconfig.yaml",
    {
      username        = each.value.user
      server          = var.test_control_plane_endpoint
      tls_ca          = base64encode(file("../control-plane/output/kubernetes-ca.pem"))
      tls_client_cert = base64encode(tls_locally_signed_cert.tls_client[each.key].cert_pem)
      tls_client_key  = base64encode(tls_private_key.tls_client[each.key].private_key_pem)
    },
  )
}

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

  filename = abspath("${path.module}/output/manifests/${each.key}.yaml")
  content = templatefile(
    "${local.k8s_manifests_path}/${each.key}.yaml",
    try(each.value.variables, {})
  )
}
