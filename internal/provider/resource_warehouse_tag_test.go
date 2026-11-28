//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccLakekeeperWarehouseTag_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	keyPrefix := acctest.RandString(8)
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)
	tagName := "env." + acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperWarehouseTagDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "lakekeeper_tag_definition" "env" {
  project_id = %q
  name       = %q
  scope      = ["warehouse"]
  value_kind = "free-text"
}

resource "lakekeeper_warehouse_tag" "foo" {
  project_id   = %q
  warehouse_id = %q
  tag_name     = lakekeeper_tag_definition.env.name
  value        = "staging"
}
`, project.ID, tagName, project.ID, warehouse.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_tag.foo", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_tag.foo", "tag_name", tagName),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_tag.foo", "value", "staging"),
					resource.TestCheckResourceAttrSet("lakekeeper_warehouse_tag.foo", "tag_definition_id"),
					resource.TestCheckResourceAttrSet("lakekeeper_warehouse_tag.foo", "applied_at"),
				),
			},
			{
				ResourceName:      "lakekeeper_warehouse_tag.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`
resource "lakekeeper_tag_definition" "env" {
  project_id = %q
  name       = %q
  scope      = ["warehouse"]
  value_kind = "free-text"
}

resource "lakekeeper_warehouse_tag" "foo" {
  project_id   = %q
  warehouse_id = %q
  tag_name     = lakekeeper_tag_definition.env.name
  value        = "production"
}
`, project.ID, tagName, project.ID, warehouse.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_tag.foo", "value", "production"),
				),
			},
		},
	})
}

func testAccCheckLakekeeperWarehouseTagDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_warehouse_tag" {
			continue
		}
		parts := strings.SplitN(rs.Primary.ID, "/", 3)
		if len(parts) != 3 {
			continue
		}
		tags, err := testutil.TestLakekeeperClient.ListWarehouseTags(context.Background(), parts[0], parts[1], false)
		if err != nil {
			if api.IsNotFound(err) {
				continue
			}
			return err
		}
		for _, tag := range tags {
			if strings.EqualFold(tag.Name, parts[2]) {
				return fmt.Errorf("warehouse tag %s still exists", rs.Primary.ID)
			}
		}
	}
	return nil
}
