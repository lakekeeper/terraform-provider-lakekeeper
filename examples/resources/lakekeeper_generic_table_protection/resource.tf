# The generic table is created outside Terraform; reference its existing UUID.
resource "lakekeeper_generic_table_protection" "images" {
  warehouse_id     = "a4653498-1dd9-4f12-a2e4-1cc7d4023226"
  generic_table_id = "2b7d4f3a-8c1e-4a5b-9f0d-1e2a3b4c5d6e"
  protected        = true
}
