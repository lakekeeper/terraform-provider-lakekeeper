resource "lakekeeper_warehouse_tag" "env" {
  project_id   = "891d18c8-1da4-471e-89f1-6e43eb4dcb38"
  warehouse_id = "261bd4e4-5c57-4707-96ad-44128f9038c0"
  tag_name     = "environment"
  value        = "production"
}
