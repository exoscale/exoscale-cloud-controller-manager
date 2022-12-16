## TLS resources

# Client certificates
resource "tls_private_key" "tls_client" {
  for_each = toset([
    "external-node",
  ])

  algorithm   = "ECDSA"
  ecdsa_curve = "P256"
}

resource "tls_cert_request" "tls_client" {
  for_each = {
    "external-node" = { user = "system:node:${exoscale_compute_instance.external_node.name}", group = "system:nodes" }
  }

  private_key_pem = tls_private_key.tls_client[each.key].private_key_pem

  subject {
    common_name  = each.value.user
    organization = each.value.group
  }
}

resource "tls_locally_signed_cert" "tls_client" {
  for_each = resource.tls_cert_request.tls_client

  cert_request_pem   = tls_cert_request.tls_client[each.key].cert_request_pem
  ca_private_key_pem = file("../control-plane/output/kubernetes-key.pem")
  ca_cert_pem        = file("../control-plane/output/kubernetes-ca.pem")

  is_ca_certificate     = false
  validity_period_hours = 168 # 7 days
  allowed_uses          = ["key_agreement", "key_encipherment", "digital_signature", "client_auth"]
}
