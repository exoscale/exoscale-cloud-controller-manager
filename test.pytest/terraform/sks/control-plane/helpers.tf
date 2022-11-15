## Helper resources

# Unique test ID (suffix)
resource "random_string" "test_id" {
  length  = 5
  upper   = false
  special = false
}
