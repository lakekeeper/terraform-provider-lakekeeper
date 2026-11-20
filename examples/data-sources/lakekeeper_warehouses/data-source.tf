# List all active warehouses in a project
data "lakekeeper_warehouses" "example" {
  project_id = "a8a6b1b8-1c0e-4c3b-9f0a-2e5d7f3c9b11"
}
