## TLS resources

# Certificate Authorities
# ("Single root CA", REF: https://kubernetes.io/docs/setup/best-practices/certificates/#single-root-ca)
resource "tls_private_key" "tls_ca" {
  for_each = toset([
    "kubernetes",
    "service-account",
    "etcd",
    "front-proxy",
  ])

  algorithm   = "ECDSA"
  ecdsa_curve = "P256"
}

resource "tls_self_signed_cert" "tls_ca" {
  for_each = resource.tls_private_key.tls_ca

  private_key_pem = tls_private_key.tls_ca[each.key].private_key_pem

  subject {
    common_name = each.key
  }

  is_ca_certificate     = true
  set_subject_key_id    = true
  validity_period_hours = 168 # 7 days
  allowed_uses          = ["cert_signing"]
}

# (cluster CA)
resource "local_file" "tls_ca" {
  for_each = resource.tls_self_signed_cert.tls_ca

  filename = abspath("${path.module}/output/${each.key}-ca.pem")
  content  = tls_self_signed_cert.tls_ca[each.key].cert_pem
}
resource "local_sensitive_file" "tls_ca" {
  for_each = resource.tls_self_signed_cert.tls_ca

  filename        = abspath("${path.module}/output/${each.key}-key.pem")
  file_permission = "0600"
  content         = tls_private_key.tls_ca[each.key].private_key_pem
}

# Client certificates
resource "tls_private_key" "tls_client" {
  for_each = toset([
    "admin",
    "ccm",
  ])

  algorithm   = "ECDSA"
  ecdsa_curve = "P256"
}

resource "tls_cert_request" "tls_client" {
  for_each = {
    "admin" = { user = "admin", group = "system:masters" }
    "ccm"   = { user = "ccm-${random_string.test_id.result}", group = "system:cloud-controller-manager" }
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
  ca_private_key_pem = tls_private_key.tls_ca["kubernetes"].private_key_pem
  ca_cert_pem        = tls_self_signed_cert.tls_ca["kubernetes"].cert_pem

  is_ca_certificate     = false
  validity_period_hours = 168 # 7 days
  allowed_uses          = ["key_agreement", "key_encipherment", "digital_signature", "client_auth"]
}
