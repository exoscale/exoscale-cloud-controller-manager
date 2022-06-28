# Kubeadm resources

## Kubelet bootstrap token

resource "random_string" "token_id" {
  length  = 6
  lower   = true
  numeric = true
  special = false
  upper   = false
}

resource "random_string" "token_secret" {
  length  = 16
  lower   = true
  numeric = true
  special = false
  upper   = false
}

## Certificate Authorities ("Single root CA", REF: https://kubernetes.io/docs/setup/best-practices/certificates/#single-root-ca)

resource "tls_private_key" "root_ca" {
  for_each = toset(["kubernetes-ca", "etcd-ca", "kubernetes-front-proxy-ca", "service-account-management"])

  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "tls_self_signed_cert" "root_ca" {
  for_each = toset(["kubernetes-ca", "etcd-ca", "kubernetes-front-proxy-ca"])

  private_key_pem = tls_private_key.root_ca[each.key].private_key_pem

  subject {
    common_name = each.key
  }

  is_ca_certificate     = true
  validity_period_hours = 240
  allowed_uses          = ["cert_signing"]
  set_subject_key_id    = true
}

data "tls_public_key" "root_ca_public_key" {
  private_key_pem = tls_private_key.root_ca["kubernetes-ca"].private_key_pem
}

### Client certificates

resource "tls_private_key" "certificate" {
  for_each = toset(["operator", "ccm"])

  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "tls_cert_request" "certificate" {
  for_each = {
    operator = { user = "admin", group = "system:masters" }
    ccm      = { user = "cloud-controller-manager", group = "cloud-controller-manager" }
  }

  private_key_pem = tls_private_key.certificate[each.key].private_key_pem

  subject {
    common_name  = each.value.user
    organization = each.value.group
  }
}

resource "tls_locally_signed_cert" "certificate" {
  for_each = toset(["operator", "ccm"])

  cert_request_pem   = tls_cert_request.certificate[each.key].cert_request_pem
  ca_private_key_pem = tls_private_key.root_ca["kubernetes-ca"].private_key_pem
  ca_cert_pem        = tls_self_signed_cert.root_ca["kubernetes-ca"].cert_pem

  is_ca_certificate = false

  validity_period_hours = 240

  allowed_uses = concat([
    "key_encipherment",
    "digital_signature",
    "client_auth"
  ])
}

# Exoscale Infrastructure

data "http" "local_ip" {
  url = "http://ipconfig.me"
}

resource "random_string" "test_id" {
  length  = 5
  upper   = false
  special = false
}

resource "exoscale_security_group" "cluster" {
  name             = local.name
  external_sources = ["${chomp(data.http.local_ip.body)}/32"]
}

resource "exoscale_security_group_rule" "cluster" {
  for_each = local.cluster_security_groups

  security_group_id      = exoscale_security_group.cluster.id
  protocol               = each.value["protocol"]
  type                   = "INGRESS"
  icmp_type              = try(each.value.icmp_type, null)
  icmp_code              = try(each.value.icmp_code, null)
  start_port             = try(split("-", each.value.port)[0], each.value.port, null)
  end_port               = try(split("-", each.value.port)[1], each.value.port, null)
  user_security_group_id = exoscale_security_group.cluster.id
}

resource "exoscale_anti_affinity_group" "cluster" {
  name = local.name
}

data "exoscale_compute_template" "ubuntu" {
  zone = var.zone
  # FIXME: control plane Pods fails under Ubuntu 22.04, when provisioning
  # them as static manifests (as Kubeadm does).
  # It's probably a bug with upstream containerd setup and conflicting updates
  # on system settings (sysctls) under Ubuntu 22.04.
  name = "Linux Ubuntu 20.04 LTS 64-bit"
}

resource "tls_private_key" "ssh_key" {
  algorithm = "ED25519"
}

resource "exoscale_ssh_key" "test" {
  name       = "${var.name}-${random_string.test_id.result}"
  public_key = tls_private_key.ssh_key.public_key_openssh
}

resource "exoscale_compute_instance" "control_plane" {
  zone               = var.zone
  name               = "${var.name}-${random_string.test_id.result}-control-plane"
  type               = "standard.medium"
  template_id        = data.exoscale_compute_template.ubuntu.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.cluster.id]
  ssh_key            = exoscale_ssh_key.test.name
  user_data          = local.control_plane_user_data


  connection {
    user        = data.exoscale_compute_template.ubuntu.username
    host        = self.public_ip_address
    private_key = tls_private_key.ssh_key.private_key_openssh
  }

  provisioner "file" {
    source      = "manifests/calico.yml"
    destination = "/tmp/calico.yml"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo cloud-init status -w",
      "sudo mv /tmp/calico.yml /root/calico.yml",
      "sudo kubectl apply --kubeconfig=/etc/kubernetes/admin.conf -f /root/calico.yml",
      "sudo kubectl wait --kubeconfig=/etc/kubernetes/admin.conf --timeout 600s node/${self.name} --for=condition=Ready"
    ]
  }
}

resource "exoscale_instance_pool" "test" {
  count              = var.pool_size > 0 ? 1 : 0
  zone               = var.zone
  name               = "${var.name}-${random_string.test_id.result}"
  instance_prefix    = "${var.name}-${random_string.test_id.result}-pool"
  size               = var.pool_size
  instance_type      = "standard.medium"
  template_id        = data.exoscale_compute_template.ubuntu.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.cluster.id]
  key_pair           = exoscale_ssh_key.test.name
  user_data          = local.node_user_data

  depends_on = [exoscale_compute_instance.control_plane]
}

resource "exoscale_compute_instance" "external" {
  zone               = var.zone
  name               = "${var.name}-${random_string.test_id.result}-external"
  type               = "standard.medium"
  template_id        = data.exoscale_compute_template.ubuntu.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.cluster.id]
  ssh_key            = exoscale_ssh_key.test.name
  user_data          = local.node_user_data

  depends_on = [exoscale_compute_instance.control_plane]
}

resource "exoscale_nlb" "external" {
  zone        = var.zone
  name        = "${var.name}-${random_string.test_id.result}"
  description = "${var.name}-${random_string.test_id.result} description"

  depends_on = [exoscale_instance_pool.test]
}

# Generated assets: SSH key & (admin / ccm)cluster authentication

resource "local_sensitive_file" "ssh_key" {
  filename        = "id_ed25519"
  content         = tls_private_key.ssh_key.private_key_openssh
  file_permission = "0600"
}

resource "local_sensitive_file" "cluster_client" {
  for_each        = tls_locally_signed_cert.certificate
  filename        = "${each.key}.kubeconfig"
  content         = <<EOT
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${base64encode(tls_self_signed_cert.root_ca["kubernetes-ca"].cert_pem)}
    server: https://${exoscale_compute_instance.control_plane.public_ip_address}:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes-admin@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
preferences: {}
users:
- name: kubernetes-admin
  user:
    client-certificate-data: ${base64encode(tls_locally_signed_cert.certificate[each.key].cert_pem)}
    client-key-data: ${base64encode(tls_private_key.certificate[each.key].private_key_pem)}
EOT
  file_permission = "0600"
}


resource "null_resource" "manifests" {
  for_each   = var.manifests
  depends_on = [local_sensitive_file.cluster_client["operator"]]

  triggers = {
    hash          = sha256(file("./manifests/${each.value}.yml"))
    apply_command = "kubectl apply -f ./manifests/${each.value}.yml"
    kubeconfig    = local_sensitive_file.cluster_client["operator"].filename
  }

  provisioner "local-exec" {
    command = self.triggers.apply_command
    environment = {
      KUBECONFIG = self.triggers.kubeconfig
    }
  }
}

resource "local_file" "sks_dev_env" {
  content  = <<-EOT
  export KUBECONFIG="${abspath(local_sensitive_file.cluster_client["operator"].filename)}"
  export CCM_KUBECONFIG="${abspath(local_sensitive_file.cluster_client["operator"].filename)}"
  export EXOSCALE_ZONE="${var.zone}"
  export EXOSCALE_SKS_AGENT_RUNNERS=node-csr-validation
  export EXOSCALE_API_CREDENTIALS_FILE=${abspath("../api-creds")}

  # Helper alias to approve pending CSRs from Kubelets
  alias approve-csr="kubectl get csr -o go-template='{{range .items}}{{if not .status}}{{.metadata.name}}{{\"\\n\"}}{{end}}{{end}}' | xargs kubectl certificate approve"

  # Helper alias to run CCM from local env on the remote cluster
  alias go-run-ccm="go run ${abspath("../../cmd/exoscale-cloud-controller-manager/main.go")} \
    --kubeconfig=$CCM_KUBECONFIG \
    --authentication-kubeconfig=$CCM_KUBECONFIG \
    --authorization-kubeconfig=$CCM_KUBECONFIG \
    --cloud-config=${abspath("../cloud-config.conf")} \
    --leader-elect=true \
    --allow-untagged-cloud  \
    --v=3"
  EOT
  filename = ".env"
}
