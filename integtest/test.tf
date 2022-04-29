terraform {
  required_providers {
    exoscale = {
      source  = "exoscale/exoscale"
      version = "~> 0.35"
    }
  }
}

variable "tmpdir" {}
variable "zone" {}
locals { test_prefix = "test-ccm" }

resource "random_string" "random" {
  length  = 5
  upper   = false
  special = false
}

data "exoscale_compute_template" "coi" {
  zone   = var.zone
  name   = "Exoscale Container-Optimized Instance"
}

data "template_file" "master_node_kubeadm_init" {
  template = <<EOF
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    cloud-provider: "external"
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
networking:
  podSubnet: "192.168.0.0/16" # --pod-network-cidr for Calico.
EOF
}

data "template_file" "master_node_userdata" {
  template = <<EOF
#cloud-config

packages:
- make

write_files:
- path: /tmp/kubeadm-init.yml
  content: ${base64encode(data.template_file.master_node_kubeadm_init.rendered)}
  encoding: b64
EOF
}

data "local_file" "kube-ca-crt" {
  filename   = "${var.tmpdir}/kube-ca.crt"
  depends_on = [exoscale_compute_instance.kube_master_node]
}

data "local_file" "cluster-endpoint" {
  filename   = "${var.tmpdir}/cluster_endpoint"
  depends_on = [exoscale_compute_instance.kube_master_node]
}

data "local_file" "kubelet-join-token" {
  filename   = "${var.tmpdir}/kubelet_join_token"
  depends_on = [exoscale_compute_instance.kube_master_node]
}

data "template_file" "kubenode_kubelet_bootstrap" {
  template = <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${data.local_file.kube-ca-crt.content}
    server: ${data.local_file.cluster-endpoint.content}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: tls-bootstrap-token-user
  name: tls-bootstrap-token-user@kubernetes
current-context: tls-bootstrap-token-user@kubernetes
users:
- name: tls-bootstrap-token-user
  user:
    token: ${data.local_file.kubelet-join-token.content}
preferences: {}
EOF

}

data "template_file" "kubenode_userdata" {
  template = <<EOF
#cloud-config

write_files:
- path: /etc/kubernetes/pki/ca.crt
  content: ${data.local_file.kube-ca-crt.content}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/kubernetes/bootstrap-kubelet.conf
  content: ${base64encode(data.template_file.kubenode_kubelet_bootstrap.rendered)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/default/kubelet
  content: "KUBELET_OPTS='--cloud-provider=external'"
  owner: root:root
EOF
}

resource "exoscale_security_group" "test" {
  name = "${local.test_prefix}-${random_string.random.result}"
}

resource "exoscale_security_group_rule" "test" {
  for_each = {
    internal_tcp       = { protocol = "TCP", port = "1-65535", sg = exoscale_security_group.test.id }
    internal_udp       = { protocol = "UDP", port = "1-65535", sg = exoscale_security_group.test.id }
    internal_ipip      = { protocol = "IPIP", sg = exoscale_security_group.test.id }
    ssh_ipv4           = { protocol = "TCP", port = 22, cidr = "0.0.0.0/0" }
    ssh_ipv6           = { protocol = "TCP", port = 22, cidr = "::/0" }
    http_ipv4          = { protocol = "TCP", port = 80, cidr = "0.0.0.0/0" }
    http_ipv6          = { protocol = "TCP", port = 80, cidr = "::/0" }
    https_ipv4         = { protocol = "TCP", port = 443, cidr = "0.0.0.0/0" }
    https_ipv6         = { protocol = "TCP", port = 443, cidr = "::/0" }
    apiserver_ipv4     = { protocol = "TCP", port = 6443, cidr = "0.0.0.0/0" }
    apiserver_ipv6     = { protocol = "TCP", port = 6443, cidr = "::/0" }
    nodeports_tcp_ipv4 = { protocol = "TCP", port = "30000-32767", cidr = "0.0.0.0/0" }
    nodeports_udp_ipv4 = { protocol = "UDP", port = "30000-32767", cidr = "0.0.0.0/0" }
    nodeports_tcp_ipv6 = { protocol = "TCP", port = "30000-32767", cidr = "::/0" }
    nodeports_udp_ipv6 = { protocol = "UDP", port = "30000-32767", cidr = "::/0" }
  }

  security_group_id      = exoscale_security_group.test.id
  protocol               = each.value.protocol
  type                   = "INGRESS"
  start_port             = try(split("-", each.value.port)[0], each.value.port, null)
  end_port               = try(split("-", each.value.port)[1], each.value.port, null)
  user_security_group_id = try(each.value.sg, null)
  cidr                   = try(each.value.cidr, null)
}

resource "tls_private_key" "ssh_key" {
  algorithm = "ED25519"
}

resource "exoscale_ssh_key" "test" {
  name       = "${local.test_prefix}-${random_string.random.result}"
  public_key = tls_private_key.ssh_key.public_key_openssh
}

resource "local_sensitive_file" "ssh_key" {
  filename        = "${var.tmpdir}/id_ed25519"
  content         = tls_private_key.ssh_key.private_key_openssh
  file_permission = "0600"
}

resource "exoscale_compute_instance" "kube_master_node" {
  zone               = var.zone
  name               = "${local.test_prefix}-${random_string.random.result}-master"
  type               = "standard.medium"
  template_id        = data.exoscale_compute_template.coi.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.test.id]
  ssh_key            = exoscale_ssh_key.test.name
  user_data          = data.template_file.master_node_userdata.rendered

  provisioner "file" {
    connection {
      type        = "ssh"
      host        = self.public_ip_address
      user        = data.exoscale_compute_template.coi.username
      private_key = tls_private_key.ssh_key.private_key_openssh
    }

    # Copy repository sources to the master node
    source      = "${path.cwd}/../"
    destination = "/home/${data.exoscale_compute_template.coi.username}/"
  }

  provisioner "remote-exec" {
    connection {
      type        = "ssh"
      host        = self.public_ip_address
      user        = data.exoscale_compute_template.coi.username
      private_key = tls_private_key.ssh_key.private_key_openssh
    }

    inline = [
      "echo '### Installing OS dependencies ...'",
      "sudo apt update && sudo apt install --yes --no-install-recommends make",
      "echo '### Building Exoscale Cloud-Controller Manager (CCM) image ...'",
      "git tag test ; make -f Makefile.docker docker",
      "echo '### Bootstrapping Kubernetes control-plane ...'",
      "sudo touch /etc/kubernetes/bootstrap-kubelet.conf",
      "sudo kubeadm init --config=/tmp/kubeadm-init.yml",
      "sudo cp /etc/kubernetes/admin.conf /tmp/kubeconfig && sudo chmod 644 /tmp/kubeconfig",
      "sudo kubeadm token create --log-file /dev/null 2>/dev/null > /tmp/kubelet_join_token",
    ]
  }

  provisioner "local-exec" {
    command = <<EOF
set -e ; \
echo '### Copying Kubernetes configuration resources ...' ; \
scp -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i "${var.tmpdir}/id_ed25519" ${data.exoscale_compute_template.coi.username}@${exoscale_compute_instance.kube_master_node.public_ip_address}:/tmp/kubeconfig ${var.tmpdir}/ ; \
scp -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i "${var.tmpdir}/id_ed25519" ${data.exoscale_compute_template.coi.username}@${exoscale_compute_instance.kube_master_node.public_ip_address}:/tmp/kubelet_join_token ${var.tmpdir}/ ; \
echo '### Dumping Kubernetes control-plane resources ...' ; \
export KUBECONFIG="${var.tmpdir}/kubeconfig" ; \
kubectl config view --raw -o jsonpath="{.clusters[].cluster.certificate-authority-data}" > "${var.tmpdir}/kube-ca.crt" ; \
kubectl config view --raw -o jsonpath="{.clusters[].cluster.server}" > "${var.tmpdir}/cluster_endpoint" ; \
echo '### Installing Calico ...' ; \
kubectl apply -f "${var.tmpdir}/manifests/calico.yml" ; \
echo '### Waiting for Kubernetes control-plane to be ready ...' ; \
kubectl wait --timeout 600s node/${exoscale_compute_instance.kube_master_node.name} --for=condition=Ready ; \
echo '### Installing Exoscale Cloud-Controller Manager (CCM) ...' ; \
kubectl apply -f "${var.tmpdir}/manifests/ccm.yml" ; \
echo '### Waiting for Exoscale Cloud-Controller Manager (CCM) to be ready ...' ; \
kubectl wait --timeout 600s -n kube-system --for condition=Available deployment.apps/exoscale-cloud-controller-manager
EOF
  }
}

resource "exoscale_instance_pool" "test" {
  zone               = var.zone
  name               = "${local.test_prefix}-${random_string.random.result}"
  instance_prefix    = "${local.test_prefix}-${random_string.random.result}-pool"
  size               = 1
  instance_type      = "standard.medium"
  template_id        = data.exoscale_compute_template.coi.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.test.id]
  key_pair           = exoscale_ssh_key.test.name
  user_data          = data.template_file.kubenode_userdata.rendered

  #depends_on = [exoscale_compute_instance.kube_master_node]
}

resource "exoscale_compute_instance" "external" {
  zone               = var.zone
  name               = "${local.test_prefix}-${random_string.random.result}-external"
  type               = "standard.medium"
  template_id        = data.exoscale_compute_template.coi.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.test.id]
  ssh_key            = exoscale_ssh_key.test.name
  user_data          = data.template_file.kubenode_userdata.rendered

  #depends_on = [exoscale_compute_instance.kube_master_node]
}

resource "exoscale_nlb" "external" {
  zone        = var.zone
  name        = "${local.test_prefix}-${random_string.random.result}"
  description = "${local.test_prefix}-${random_string.random.result} description"

  depends_on = [exoscale_instance_pool.test]
}

output "test_id" { value = random_string.random.result }
output "master_node_id" { value = exoscale_compute_instance.kube_master_node.id }
output "master_node_ip" { value = exoscale_compute_instance.kube_master_node.public_ip_address }
output "nodepool_id" { value = exoscale_instance_pool.test.id }
output "external_node_id" { value = exoscale_compute_instance.external.id }
output "external_node_ip" { value = exoscale_compute_instance.external.public_ip_address }
output "external_nlb_id" { value = exoscale_nlb.external.id }
output "external_nlb_ip" { value = exoscale_nlb.external.ip_address }
output "external_nlb_name" { value = exoscale_nlb.external.name }
output "external_nlb_desc" { value = exoscale_nlb.external.description }
