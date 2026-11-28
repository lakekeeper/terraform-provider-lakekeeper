//go:build acceptance

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccDataLakekeeperTagDefinition_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, acctest.RandString(8))
	tagName := "ds." + acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "lakekeeper_tag_definition" "tag" {
  project_id = %q
  name       = %q
  scope      = ["warehouse"]
  value_kind = "marker"
}

resource "lakekeeper_warehouse_tag" "wh" {
  project_id   = %q
  warehouse_id = %q
  tag_name     = lakekeeper_tag_definition.tag.name
}

data "lakekeeper_tag_definition" "by_name" {
  project_id = %q
  name       = lakekeeper_tag_definition.tag.name
}

data "lakekeeper_tag_definitions" "all" {
  project_id = %q
  depends_on = [lakekeeper_tag_definition.tag]
}

data "lakekeeper_tag_attachments" "att" {
  project_id        = %q
  tag_definition_id = lakekeeper_tag_definition.tag.tag_definition_id
  depends_on        = [lakekeeper_warehouse_tag.wh]
}

data "lakekeeper_effective_tags" "eff" {
  project_id   = %q
  warehouse_id = %q
  target_type  = "warehouse"
  depends_on   = [lakekeeper_warehouse_tag.wh]
}
`, project.ID, tagName, project.ID, warehouse.ID, project.ID, project.ID, project.ID, project.ID, warehouse.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.lakekeeper_tag_definition.by_name", "tag_definition_id", "lakekeeper_tag_definition.tag", "tag_definition_id"),
					resource.TestCheckResourceAttr("data.lakekeeper_tag_definitions.all", "definitions.#", "1"),
					resource.TestCheckResourceAttr("data.lakekeeper_tag_attachments.att", "attachments.#", "1"),
					resource.TestCheckResourceAttr("data.lakekeeper_effective_tags.eff", "tags.#", "1"),
				),
			},
		},
	})
}
