resource "lakekeeper_column_tag" "email_pii" {
  warehouse_id = "261bd4e4-5c57-4707-96ad-44128f9038c0"
  table_id     = "37cf8760-5092-4d5d-8b29-a6b46a45f067"
  column_name  = "email"
  tag_name     = "pii"
}
