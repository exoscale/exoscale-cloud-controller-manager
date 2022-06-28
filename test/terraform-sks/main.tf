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

resource "exoscale_sks_cluster" "cluster" {
  zone    = var.zone
  name    = local.name
  version = try(var.sks_version, null)

  cni            = "calico"
  exoscale_ccm   = false
  metrics_server = true
}

resource "exoscale_sks_kubeconfig" "client" {
  for_each = {
    operator = { user = "admin", groups = ["system:masters"] }
    ccm      = { user = "cloud-controller-manager", groups = ["system:cloud-controller-manager"] }
  }

  zone       = var.zone
  cluster_id = exoscale_sks_cluster.cluster.id

  user        = "admin"
  groups      = ["system:masters"]
  ttl_seconds = 2628000
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

resource "local_sensitive_file" "cluster_client" {
  for_each        = exoscale_sks_kubeconfig.client
  content         = exoscale_sks_kubeconfig.client[each.key].kubeconfig
  filename        = "${each.key}.kubeconfig"
  file_permission = "0600"
}

resource "exoscale_sks_nodepool" "pool" {
  count         = var.pool_size > 0 ? 1 : 0
  zone          = var.zone
  cluster_id    = exoscale_sks_cluster.cluster.id
  name          = local.name
  instance_type = "standard.medium"
  size          = var.pool_size

  anti_affinity_group_ids = [exoscale_anti_affinity_group.cluster.id]
  security_group_ids      = [exoscale_security_group.cluster.id]
}

resource "null_resource" "manifests" {
  for_each   = var.manifests
  depends_on = [exoscale_sks_kubeconfig.client]

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
