//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"math/rand"
	"regexp"
	"testing"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperWarehouseRoleAssignment_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	keyPrefix := fmt.Sprintf("key-prefix-%d", rand.Int())
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)

	role := testutil.CreateRole(t, project.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperWarehouseRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_role_assignment" "test" {
						warehouse_id = "%s"
						role_id = "%s"
						assignments = ["ownership"]
					}
				`, warehouse.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "id", warehouse.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.0", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// change one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_role_assignment" "test" {
						warehouse_id = "%s"
						role_id = "%s"
						assignments = ["modify"]
					}
				`, warehouse.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "id", warehouse.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.0", "modify"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// add one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_role_assignment" "test" {
						warehouse_id = "%s"
						role_id = "%s"
						assignments = ["ownership", "manage_grants", "create"]
					}
				`, warehouse.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "id", warehouse.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.#", "3"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.0", "create"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.1", "manage_grants"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.2", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Check some basic attribute validation
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_warehouse_role_assignment" "test" {
						warehouse_id = "%s/%s"
						role_id = "%s"
						assignments = ["ownership"]
					}
				`, project.ID, warehouse.ID, role.ID),
				ExpectError: regexp.MustCompile("Attribute warehouse_id must be a UUID"),
			},
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_warehouse_role_assignment" "test" {
						warehouse_id = "%s"
						role_id = "%s/%s"
						assignments = ["ownership"]
					}
				`, warehouse.ID, project.ID, role.ID),
				ExpectError: regexp.MustCompile("Attribute role_id must be a UUID"),
			},
			// delete all assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_role_assignment" "test" {
						warehouse_id = "%s"
						role_id = "%s"
						assignments = []
					}
				`, warehouse.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "id", warehouse.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.#", "0"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckLakekeeperWarehouseRoleAssignmentDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_warehouse_role_assignment" {
			continue
		}

		parts, err := warehouseRoleAssignment.parseID(rs.Primary.ID)
		if err != nil {
			return err
		}
		warehouseID, roleID := parts[0], parts[1]

		assignments, err := testutil.TestLakekeeperClient.GetWarehouseAssignments(context.Background(), warehouseID)
		if err != nil {
			return fmt.Errorf("could not list warehouse assignments to check destroy, %w", err)
		}

		for _, v := range assignments {
			if v.Assignee.ID == roleID && v.Assignee.Type == api.AssigneeTypeRole {
				return fmt.Errorf("warehouse assignment still exists")
			}
		}
	}

	return nil
}

// Lakekeeper 0.15 added the `write` privilege. It must be accepted, read back
// from the server and stay stable across plans (no drift, no revocation).
func TestAccLakekeeperWarehouseRoleAssignment_write(t *testing.T) {
	project := testutil.CreateProject(t)

	keyPrefix := fmt.Sprintf("key-prefix-%d", rand.Int())
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)

	role := testutil.CreateRole(t, project.ID)

	config := fmt.Sprintf(`
		resource "lakekeeper_warehouse_role_assignment" "test" {
			warehouse_id = "%s"
			role_id = "%s"
			assignments = ["describe", "select", "write"]
		}
	`, warehouse.ID, role.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperWarehouseRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.#", "3"),
					resource.TestCheckTypeSetElemAttr("lakekeeper_warehouse_role_assignment.test", "assignments.*", "write"),
				),
			},
			// The same config must plan clean: write is read back, not revoked.
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:      "lakekeeper_warehouse_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// The tag grant `manage_tags` must be accepted, read back and stay stable
// across plans, like every other warehouse grant.
func TestAccLakekeeperWarehouseRoleAssignment_manageTags(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))
	role := testutil.CreateRole(t, project.ID)

	config := fmt.Sprintf(`
		resource "lakekeeper_warehouse_role_assignment" "test" {
			warehouse_id = "%s"
			role_id = "%s"
			assignments = ["describe", "manage_tags"]
		}
	`, warehouse.ID, role.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperWarehouseRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_role_assignment.test", "assignments.#", "2"),
					resource.TestCheckTypeSetElemAttr("lakekeeper_warehouse_role_assignment.test", "assignments.*", "manage_tags"),
				),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:      "lakekeeper_warehouse_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
