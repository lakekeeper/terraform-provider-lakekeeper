resource "lakekeeper_tag_role_assignment" "stewards" {
  tag_definition_id = "261bd4e4-5c57-4707-96ad-44128f9038c0"
  role_id           = "37cf8760-5092-4d5d-8b29-a6b46a45f067"
  assignments       = ["apply", "ownership"]
}
