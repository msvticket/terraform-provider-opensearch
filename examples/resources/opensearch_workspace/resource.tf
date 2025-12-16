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

# Create a workspace with permissions
resource "opensearch_workspace" "secured" {
  name        = "secured-workspace"
  description = "Workspace with access controls"
  features    = ["discover", "visualize"]
  
  permissions {
    library_write {
      users  = ["admin_user", "power_user"]
      groups = ["admin_group"]
    }
    
    library_read {
      users  = ["readonly_user"]
      groups = ["readonly_group"]
    }
  }
}
