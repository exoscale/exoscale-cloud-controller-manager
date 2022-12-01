## SSH resources

# Key
resource "tls_private_key" "ssh_key" {
  algorithm = "ED25519"
}

resource "local_sensitive_file" "ssh_key" {
  filename        = abspath("${path.module}/output/ssh.id_ed25519")
  content         = tls_private_key.ssh_key.private_key_openssh
  file_permission = "0600"
}

resource "exoscale_ssh_key" "ssh_key" {
  name = local.test_name

  public_key = tls_private_key.ssh_key.public_key_openssh
}
