resource "lakekeeper_tag_definition" "pii" {
  project_id     = "261bd4e4-5c57-4707-96ad-44128f9038c0"
  name           = "pii.classification"
  description    = "PII sensitivity classification"
  scope          = ["table", "column"]
  value_kind     = "enumerated"
  allowed_values = ["low", "high"]
}
