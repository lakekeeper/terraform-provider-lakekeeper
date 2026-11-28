resource "lakekeeper_tag_user_assignment" "alice" {
  tag_definition_id = "261bd4e4-5c57-4707-96ad-44128f9038c0"
  user_id           = "oidc~alice"
  assignments       = ["apply"]
}
