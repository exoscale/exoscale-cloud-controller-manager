terraform {
  required_providers {
    exoscale = {
      source  = "exoscale/exoscale"
      version = "~> 0.22.0"
    }
  }
}

variable "tmpdir" {}
variable "zone" {}
locals { test_prefix = "test-k8s-ccm" }

resource "random_string" "random" {
  length  = 16
  upper   = false
  special = false
}

data "exoscale_compute_template" "coi" {
  zone   = var.zone
  name   = "Container-Optimized Instance"
  filter = "community"
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
  depends_on = [exoscale_compute.kube_master_node]
}

data "local_file" "cluster-endpoint" {
  filename   = "${var.tmpdir}/cluster_endpoint"
  depends_on = [exoscale_compute.kube_master_node]
}

data "local_file" "kubelet-join-token" {
  filename   = "${var.tmpdir}/kubelet_join_token"
  depends_on = [exoscale_compute.kube_master_node]
}

data "template_file" "nodepool_kubelet_bootstrap" {
  template = <<EOF
apiVersion: v1
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
kind: Config
preferences: {}
users:
- name: tls-bootstrap-token-user
  user:
    token: ${data.local_file.kubelet-join-token.content}
EOF

}

data "template_file" "nodepool_userdata" {
  template = <<EOF
#cloud-config

write_files:
- path: /etc/kubernetes/pki/ca.crt
  content: ${data.local_file.kube-ca-crt.content}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/kubernetes/bootstrap-kubelet.conf
  content: ${base64encode(data.template_file.nodepool_kubelet_bootstrap.rendered)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/default/kubelet
  # KUBELET_OPTS="--cloud-provider=external"
  content: S1VCRUxFVF9PUFRTPSItLWNsb3VkLXByb3ZpZGVyPWV4dGVybmFsIgo=
  encoding: b64
  owner: root:root
EOF
}

resource "exoscale_security_group" "test" {
  name = "${local.test_prefix}-${random_string.random.result}"
}

resource "exoscale_security_group_rules" "test" {
  security_group = exoscale_security_group.test.name

  ingress {
    protocol                 = "TCP"
    ports                    = ["1-65535"]
    user_security_group_list = [exoscale_security_group.test.name]
  }

  ingress {
    protocol  = "TCP"
    ports     = ["22"]
    cidr_list = ["0.0.0.0/0", "::/0"]
  }

  ingress {
    protocol  = "TCP"
    ports     = ["80"]
    cidr_list = ["0.0.0.0/0", "::/0"]
  }

  ingress {
    protocol  = "TCP"
    ports     = ["443"]
    cidr_list = ["0.0.0.0/0", "::/0"]
  }

  ingress {
    protocol  = "TCP"
    cidr_list = ["0.0.0.0/0", "::/0"]
    ports     = ["6443"]
  }

  ingress {
    protocol  = "TCP"
    cidr_list = ["0.0.0.0/0"]
    ports     = ["30000-32767"]
  }
}

resource "exoscale_ssh_keypair" "test" {
  name = "${local.test_prefix}-${random_string.random.result}"
}

resource "local_file" "ssh_key" {
  filename          = "${var.tmpdir}/id_rsa"
  sensitive_content = exoscale_ssh_keypair.test.private_key
  file_permission   = "0600"
}

resource "exoscale_compute" "kube_master_node" {
  zone               = var.zone
  display_name       = "${local.test_prefix}-${random_string.random.result}"
  size               = "Medium"
  template_id        = data.exoscale_compute_template.coi.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.test.id]
  key_pair           = exoscale_ssh_keypair.test.name
  user_data          = data.template_file.master_node_userdata.rendered

  provisioner "file" {
    connection {
      type        = "ssh"
      host        = self.ip_address
      user        = data.exoscale_compute_template.coi.username
      private_key = exoscale_ssh_keypair.test.private_key
    }

    # Copy repository sources to the master node
    source      = "${path.cwd}/../"
    destination = "/home/${data.exoscale_compute_template.coi.username}/"
  }

  provisioner "remote-exec" {
    connection {
      type        = "ssh"
      host        = self.ip_address
      user        = data.exoscale_compute_template.coi.username
      private_key = exoscale_ssh_keypair.test.private_key
    }

    inline = [
      "timeout 60s bash -c 'until which kubeadm > /dev/null; do sleep 5s ; done || (echo kubeadm command not found ; exit 1)'",
      "git tag test ; make docker",
      "sudo touch /etc/kubernetes/bootstrap-kubelet.conf",
      "sudo kubeadm init --config=/tmp/kubeadm-init.yml",
      "sudo cp /etc/kubernetes/admin.conf /tmp/kubeconfig && sudo chmod 644 /tmp/kubeconfig",
      "sudo kubeadm token create --log-file /dev/null 2>/dev/null > /tmp/kubelet_join_token",
    ]
  }

  provisioner "local-exec" {
    command = <<EOF
set -e ; \
scp -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i "${var.tmpdir}/id_rsa" ${data.exoscale_compute_template.coi.username}@${exoscale_compute.kube_master_node.ip_address}:/tmp/kubeconfig ${var.tmpdir}/ ; \
scp -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i "${var.tmpdir}/id_rsa" ${data.exoscale_compute_template.coi.username}@${exoscale_compute.kube_master_node.ip_address}:/tmp/kubelet_join_token ${var.tmpdir}/ ; \
export KUBECONFIG="${var.tmpdir}/kubeconfig" ; \
kubectl config view --raw -o jsonpath="{.clusters[].cluster.certificate-authority-data}" > "${var.tmpdir}/kube-ca.crt" ; \
kubectl config view --raw -o jsonpath="{.clusters[].cluster.server}" > "${var.tmpdir}/cluster_endpoint" ; \
kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml ; \
kubectl wait --timeout 600s node/${exoscale_compute.kube_master_node.name} --for=condition=Ready ; \
sed -r -e "s/%%EXOSCALE_ZONE%%/${var.zone}/" ${path.cwd}/manifests/ccm.yml.tpl | kubectl apply -f - ; \
kubectl wait --timeout 600s -n kube-system --for condition=Available deployment.apps/exoscale-cloud-controller-manager
EOF
  }
}

resource "exoscale_instance_pool" "test" {
  zone               = var.zone
  name               = "${local.test_prefix}-${random_string.random.result}"
  size               = 1
  service_offering   = "medium"
  template_id        = data.exoscale_compute_template.coi.id
  disk_size          = 15
  security_group_ids = [exoscale_security_group.test.id]
  key_pair           = exoscale_ssh_keypair.test.name
  user_data          = data.template_file.nodepool_userdata.rendered

  depends_on = [exoscale_compute.kube_master_node]
}

resource "exoscale_nlb" "external" {
  zone        = var.zone
  name        = "${local.test_prefix}-${random_string.random.result}"
  description = "${local.test_prefix}-${random_string.random.result} description"

  depends_on = [exoscale_instance_pool.test]
}

output "test_id" { value = random_string.random.result }
output "master_node_name" { value = exoscale_compute.kube_master_node.name }
output "master_node_ip" { value = exoscale_compute.kube_master_node.ip_address }
output "nodepool_id" { value = exoscale_instance_pool.test.id }
output "external_nlb_id" { value = exoscale_nlb.external.id }
output "external_nlb_ip" { value = exoscale_nlb.external.ip_address }
output "external_nlb_name" { value = exoscale_nlb.external.name }
output "external_nlb_desc" { value = exoscale_nlb.external.description }
