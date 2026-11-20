//go:build acceptance

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccDataLakekeeperRole_byName(t *testing.T) {
	project := testutil.CreateProject(t)
	roleName := "test-role-" + acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataLakekeeperRoleConfigByName(project.ID, roleName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.lakekeeper_role.by_name", "project_id", project.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_role.by_name", "name", roleName),
					resource.TestCheckResourceAttrSet("data.lakekeeper_role.by_name", "role_id"),
					resource.TestCheckResourceAttrSet("data.lakekeeper_role.by_name", "id"),
				),
			},
		},
	})
}

func TestAccDataLakekeeperRole_byID(t *testing.T) {
	project := testutil.CreateProject(t)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataLakekeeperRoleConfigByID(project.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.lakekeeper_role.by_id", "project_id", project.ID),
					resource.TestCheckResourceAttrSet("data.lakekeeper_role.by_id", "role_id"),
					resource.TestCheckResourceAttrSet("data.lakekeeper_role.by_id", "id"),
				),
			},
		},
	})
}

func testAccDataLakekeeperRoleConfigByName(projectID, roleName string) string {
	return fmt.Sprintf(`
resource "lakekeeper_role" "test" {
  project_id = %[1]q
  name       = %[2]q
}

data "lakekeeper_role" "by_name" {
  project_id = %[1]q
  name       = lakekeeper_role.test.name
}
`, projectID, roleName)
}

func testAccDataLakekeeperRoleConfigByID(projectID string) string {
	return fmt.Sprintf(`
resource "lakekeeper_role" "test" {
  project_id = %[1]q
  name       = "test-role-by-id"
}

data "lakekeeper_role" "by_id" {
  project_id = %[1]q
  role_id    = lakekeeper_role.test.role_id
}
`, projectID)
}
