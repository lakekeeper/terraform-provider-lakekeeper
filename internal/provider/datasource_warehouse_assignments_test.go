//go:build acceptance

package provider

import (
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccDataLakekeeperWarehouseAssignments_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	user := testutil.CreateUser(t, fmt.Sprintf("oidc~%s", acctest.RandString(8)))
	role := testutil.CreateRole(t, project.ID)

	keyPrefix := acctest.RandString(8)
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)

	// assignment 2
	if err := testutil.TestLakekeeperClient.UpdateWarehouseAssignments(
		t.Context(),
		warehouse.ID,
		[]api.Assignment{
			{
				Assignee:   api.UserAssignee(user.ID),
				Assignment: api.WarehouseAssignmentOwnership,
			}, {
				Assignee:   api.RoleAssignee(role.ID),
				Assignment: api.WarehouseAssignmentDescribe,
			},
		},
		nil,
	); err != nil {
		t.Fatalf("could not create warehouse assignments, %v", err)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				data "lakekeeper_warehouse_assignments" "foo" {
					id = "%s"
				}`, warehouse.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "id", warehouse.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.#", "3"), // also have the admin user

					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.0.assignee_id", testutil.DefaultUserID),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.0.assignee_type", api.AssigneeTypeUser),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.0.assignment", api.WarehouseAssignmentOwnership),

					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.1.assignee_id", user.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.1.assignee_type", api.AssigneeTypeUser),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.1.assignment", api.WarehouseAssignmentOwnership),

					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.2.assignee_id", role.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.2.assignee_type", api.AssigneeTypeRole),
					resource.TestCheckResourceAttr("data.lakekeeper_warehouse_assignments.foo", "assignments.2.assignment", api.WarehouseAssignmentDescribe),
				),
			},
		},
	})
}
