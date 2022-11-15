## Helper resources

# Unique test ID (suffix)
resource "random_string" "test_id" {
  length  = 5
  upper   = false
  special = false
}

# Kubernetes/kubelet bootstrap token
resource "random_string" "bootstrap_token_id" {
  length  = 6
  lower   = true
  numeric = true
  special = false
  upper   = false
}

resource "random_string" "bootstrap_token_secret" {
  length  = 16
  lower   = true
  numeric = true
  special = false
  upper   = false
}
