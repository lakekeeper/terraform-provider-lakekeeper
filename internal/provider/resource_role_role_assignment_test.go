//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"regexp"
	"testing"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperRoleRoleAssignment_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	assignee := testutil.CreateRole(t, project.ID)
	role := testutil.CreateRole(t, project.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperRoleRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s"
						assignee_id = "%s"
						assignments = ["ownership"]
					}
				`, role.ID, assignee.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "id", role.ID+"/"+assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignee_id", assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.0", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// change one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s"
						assignee_id = "%s"
						assignments = ["assignee"]
					}
				`, role.ID, assignee.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "id", role.ID+"/"+assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignee_id", assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.0", "assignee"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// add one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s"
						assignee_id = "%s"
						assignments = ["ownership", "assignee"]
					}
				`, role.ID, assignee.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "id", role.ID+"/"+assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignee_id", assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.#", "2"),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.0", "assignee"),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.1", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Check some basic attribute validation
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s/%s"
						assignee_id = "%s"
						assignments = ["ownership"]
					}
				`, project.ID, role.ID, assignee.ID),
				ExpectError: regexp.MustCompile("Attribute role_id must be a UUID"),
			},
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s"
						assignee_id = "%s/%s"
						assignments = ["ownership"]
					}
				`, role.ID, project.ID, assignee.ID),
				ExpectError: regexp.MustCompile("Attribute assignee_id must be a UUID"),
			},
			// delete all assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s"
						assignee_id = "%s"
						assignments = []
					}
				`, role.ID, assignee.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "id", role.ID+"/"+assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignee_id", assignee.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_role_assignment.test", "assignments.#", "0"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Lakekeeper rejects assigning a role to itself; the provider surfaces its message.
func TestAccLakekeeperRoleRoleAssignment_selfAssignment(t *testing.T) {
	project := testutil.CreateProject(t)
	role := testutil.CreateRole(t, project.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperRoleRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_role_role_assignment" "test" {
						role_id = "%s"
						assignee_id = "%s"
						assignments = ["assignee"]
					}
				`, role.ID, role.ID),
				ExpectError: regexp.MustCompile(`(?s)Unable to create role grants of the role.*Cannot assign\s+\S+\s+to\s+itself`),
			},
		},
	})
}

func testAccCheckLakekeeperRoleRoleAssignmentDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_role_role_assignment" {
			continue
		}

		parts, err := roleRoleAssignment.parseID(rs.Primary.ID)
		if err != nil {
			return err
		}
		roleID, assigneeID := parts[0], parts[1]

		assignments, err := testutil.TestLakekeeperClient.GetRoleAssignments(context.Background(), roleID)
		if err != nil {
			return fmt.Errorf("could not list role assignments to check destroy, %w", err)
		}

		for _, v := range assignments {
			if v.Assignee.ID == assigneeID && v.Assignee.Type == api.AssigneeTypeRole {
				return fmt.Errorf("role assignment still exists")
			}
		}
	}

	return nil
}
