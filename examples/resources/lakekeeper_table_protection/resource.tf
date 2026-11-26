# The table is created outside Terraform; reference its existing UUID.
resource "lakekeeper_table_protection" "orders" {
  warehouse_id = "a4653498-1dd9-4f12-a2e4-1cc7d4023226"
  table_id     = "9ad1e2a0-5df3-4c05-8e5e-7a1f2fe7c2f1"
  protected    = true
}
