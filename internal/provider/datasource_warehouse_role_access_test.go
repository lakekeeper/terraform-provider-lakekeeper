//go:build acceptance

package provider

import (
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccDataLakekeeperWarehouseRoleAccess_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	role := testutil.CreateRole(t, project.ID)

	keyPrefix := acctest.RandString(8)
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)

	// assignment
	if err := testutil.TestLakekeeperClient.UpdateWarehouseAssignments(
		t.Context(),
		warehouse.ID,
		[]api.Assignment{
			{
				Assignee:   api.RoleAssignee(role.ID),
				Assignment: api.WarehouseAssignmentCreate,
			},
		},
		nil,
	); err != nil {
		t.Fatalf("could not create warehouse access, %v", err)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				data "lakekeeper_warehouse_role_access" "foo" {
					warehouse_id = "%s"
					role_id = "%s"
				}`, warehouse.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_role_access.foo", "id", fmt.Sprintf("%s/%s", warehouse.ID, role.ID)),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_role_access.foo", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_role_access.foo", "role_id", role.ID),
					resource.TestCheckResourceAttrSet("data.lakekeeper_warehouse_role_access.foo", "allowed_actions.#"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.lakekeeper_warehouse_role_access.foo",
						tfjsonpath.Path(
							tfjsonpath.New("allowed_actions"),
						),
						knownvalue.SetPartial([]knownvalue.Check{
							knownvalue.StringFunc(func(v string) error {
								if !slices.Contains([]string{
									api.WarehouseActionCreateNamespace,
									api.WarehouseActionGetConfig,
									api.WarehouseActionGetMetadata,
									api.WarehouseActionListNamespaces,
									api.WarehouseActionIncludeInList,
									api.WarehouseActionListDeletedTabulars,
									api.WarehouseActionGetAllTasks,
									api.WarehouseActionGetEndpointStatistics,
								}, v) {
									return fmt.Errorf("%s is not an allowed action", v)
								}
								return nil
							}),
						}),
					),
				},
			},
		},
	})
}
