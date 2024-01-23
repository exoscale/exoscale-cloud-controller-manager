## Data sources

# Nodes
data "exoscale_template" "node_template" {
  zone = var.exoscale_zone
  name = var.exoscale_instance_template
}
