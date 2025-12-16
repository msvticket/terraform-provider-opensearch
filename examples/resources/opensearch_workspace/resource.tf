# Create a workspace with all options
resource "opensearch_workspace" "analytics" {
  name        = "analytics-workspace"
  description = "Workspace for analytics team"
  features    = ["discover", "visualize", "dashboard"]
}

# Create a simple workspace with only required fields
resource "opensearch_workspace" "simple" {
  name = "simple-workspace"
}
