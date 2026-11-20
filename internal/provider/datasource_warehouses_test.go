//go:build acceptance

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccDataLakekeeperWarehouses_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, "test-data-warehouses")

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "lakekeeper_warehouses" "test" {
  project_id = %q
}
`, project.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.lakekeeper_warehouses.test", "project_id", project.ID),
					resource.TestCheckResourceAttrSet("data.lakekeeper_warehouses.test", "warehouses.#"),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouses.test", "warehouses.0.id", warehouse.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouses.test", "warehouses.0.name", warehouse.Name),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouses.test", "warehouses.0.protected", "false"),
				),
			},
		},
	})
}
