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

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperWarehouseUserAssignment_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	keyPrefix := fmt.Sprintf("key-prefix-%d", rand.Int())
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)

	userID := fmt.Sprintf("oidc~%s", acctest.RandString(32))
	user := testutil.CreateUser(t, userID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperWarehouseUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_user_assignment" "test" {
						warehouse_id = "%s"
						user_id = "%s"
						assignments = ["ownership"]
					}
				`, warehouse.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "id", warehouse.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.0", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// change one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_user_assignment" "test" {
						warehouse_id = "%s"
						user_id = "%s"
						assignments = ["select"]
					}
				`, warehouse.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "id", warehouse.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.0", "select"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// add one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_user_assignment" "test" {
						warehouse_id = "%s"
						user_id = "%s"
						assignments = ["select", "describe"]
					}
				`, warehouse.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "id", warehouse.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.#", "2"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.0", "describe"),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.1", "select"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Check some basic attribute validation
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_warehouse_user_assignment" "test" {
						warehouse_id = "%s/%s"
						user_id = "%s"
						assignments = ["ownership"]
					}
				`, project.ID, warehouse.ID, user.ID),
				ExpectError: regexp.MustCompile("Attribute warehouse_id must be a UUID"),
			},
			// delete all assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_warehouse_user_assignment" "test" {
						warehouse_id = "%s"
						user_id = "%s"
						assignments = []
					}
				`, warehouse.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "id", warehouse.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "warehouse_id", warehouse.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.#", "0"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_warehouse_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakekeeperWarehouseUserAssignment_userIDWithSlash(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))

	userID := fmt.Sprintf("oidc~%s/%s", acctest.RandString(8), acctest.RandString(8))
	user := testutil.CreateUser(t, userID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperWarehouseUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_warehouse_user_assignment" "test" {
						warehouse_id = "%s"
						user_id = "%s"
						assignments = ["describe"]
					}
				`, warehouse.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "id", warehouse.ID+"/"+userID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "user_id", userID),
					resource.TestCheckResourceAttr("lakekeeper_warehouse_user_assignment.test", "assignments.#", "1"),
				),
			},
			{
				ResourceName:      "lakekeeper_warehouse_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckLakekeeperWarehouseUserAssignmentDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_warehouse_user_assignment" {
			continue
		}

		parts, err := warehouseUserAssignment.parseID(rs.Primary.ID)
		if err != nil {
			return err
		}
		warehouseID, userID := parts[0], parts[1]

		assignments, err := testutil.TestLakekeeperClient.GetWarehouseAssignments(context.Background(), warehouseID)
		if err != nil {
			return fmt.Errorf("could not list warehouse assignments to check destroy, %w", err)
		}

		for _, v := range assignments {
			if v.Assignee.ID == userID && v.Assignee.Type == api.AssigneeTypeUser {
				return fmt.Errorf("warehouse assignment still exists")
			}
		}
	}

	return nil
}
