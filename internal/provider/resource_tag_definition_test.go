//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccLakekeeperTagDefinition_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	rName := "pii." + acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperTagDefinitionDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "lakekeeper_tag_definition" "foo" {
  project_id     = %q
  name           = %q
  description    = "Initial description"
  scope          = ["table"]
  value_kind     = "enumerated"
  allowed_values = ["low", "medium"]
}
`, project.ID, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "name", rName),
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "value_kind", "enumerated"),
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "scope.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "allowed_values.#", "2"),
					resource.TestCheckResourceAttrSet("lakekeeper_tag_definition.foo", "id"),
					resource.TestCheckResourceAttrSet("lakekeeper_tag_definition.foo", "tag_definition_id"),
					resource.TestCheckResourceAttrSet("lakekeeper_tag_definition.foo", "created_at"),
				),
			},
			{
				ResourceName:      "lakekeeper_tag_definition.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`
resource "lakekeeper_tag_definition" "foo" {
  project_id     = %q
  name           = %q
  description    = "Updated description"
  scope          = ["table", "column"]
  value_kind     = "enumerated"
  allowed_values = ["low", "medium", "high"]
}
`, project.ID, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "description", "Updated description"),
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "scope.#", "2"),
					resource.TestCheckResourceAttr("lakekeeper_tag_definition.foo", "allowed_values.#", "3"),
				),
			},
			{
				ResourceName:      "lakekeeper_tag_definition.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckLakekeeperTagDefinitionDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_tag_definition" {
			continue
		}
		projectID, tagDefID, err := splitInternalID(types.StringValue(rs.Primary.ID))
		if err != nil {
			return err
		}
		_, err = testutil.TestLakekeeperClient.GetTagDefinition(context.Background(), projectID, tagDefID)
		if err == nil {
			return fmt.Errorf("tag definition %s still exists", rs.Primary.ID)
		}
		if !api.IsNotFound(err) {
			return err
		}
	}
	return nil
}
