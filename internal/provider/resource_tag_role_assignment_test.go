//go:build acceptance

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccLakekeeperTagRoleAssignment_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	role := testutil.CreateRole(t, project.ID)
	tagName := "grant." + acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "lakekeeper_tag_definition" "tag" {
  project_id = %q
  name       = %q
  scope      = ["table"]
  value_kind = "marker"
}

resource "lakekeeper_tag_role_assignment" "foo" {
  tag_definition_id = lakekeeper_tag_definition.tag.tag_definition_id
  role_id           = %q
  assignments       = ["apply"]
}
`, project.ID, tagName, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_tag_role_assignment.foo", "assignments.#", "1"),
					resource.TestCheckTypeSetElemAttr("lakekeeper_tag_role_assignment.foo", "assignments.*", "apply"),
				),
			},
			{
				ResourceName:      "lakekeeper_tag_role_assignment.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
