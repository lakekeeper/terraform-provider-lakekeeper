# The view is created outside Terraform; reference its existing UUID.
resource "lakekeeper_view_protection" "revenue" {
  warehouse_id = "a4653498-1dd9-4f12-a2e4-1cc7d4023226"
  view_id      = "6f2c1c8e-3b4a-4c8e-9d1e-2f6a7b8c9d0e"
  protected    = true
}
