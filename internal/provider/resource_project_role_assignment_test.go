//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"regexp"
	"testing"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperProjectRoleAssignment_basic(t *testing.T) {

	project := testutil.CreateProject(t)
	role := testutil.CreateRole(t, project.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperProjectRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_project_role_assignment" "test" {
						project_id = "%s"
						role_id = "%s"
						assignments = ["data_admin"]
					}
				`, project.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "id", project.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "project_id", project.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.0", "data_admin"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_project_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// change one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_project_role_assignment" "test" {
						project_id = "%s"
						role_id = "%s"
						assignments = ["security_admin"]
					}
				`, project.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "id", project.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "project_id", project.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.0", "security_admin"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_project_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// add one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_project_role_assignment" "test" {
						project_id = "%s"
						role_id = "%s"
						assignments = ["security_admin", "describe"]
					}
				`, project.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "id", project.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "project_id", project.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.#", "2"),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.0", "describe"),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.1", "security_admin"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_project_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Check some basic attribute validation
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_project_role_assignment" "test" {
						project_id = "%s"
						role_id = "%s/%s"
						assignments = ["describe"]
					}
				`, project.ID, project.ID, role.ID),
				ExpectError: regexp.MustCompile("Attribute role_id must be a UUID"),
			},
			// delete all assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_project_role_assignment" "test" {
						project_id = "%s"
						role_id = "%s"
						assignments = []
					}
				`, project.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "id", project.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "project_id", project.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.#", "0"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_project_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Create refuses grants the role already holds on the project; importing the
// assignment first brings them under Terraform.
func TestAccLakekeeperProjectRoleAssignment_existingGrants(t *testing.T) {
	project := testutil.CreateProject(t)
	role := testutil.CreateRole(t, project.ID)

	_, err := testutil.TestLakekeeperClient.PermissionV1().ProjectPermission().Update(t.Context(), project.ID, &permissionv1.UpdateProjectPermissionsOptions{
		Writes: []*permissionv1.ProjectAssignment{{
			Assignee:   permissionv1.UserOrRole{Type: permissionv1.RoleType, Value: role.ID},
			Assignment: permissionv1.DescribeProjectAssignment,
		}},
	})
	if err != nil {
		t.Fatalf("could not grant describe to the role: %v", err)
	}

	config := fmt.Sprintf(`
		resource "lakekeeper_project_role_assignment" "test" {
			project_id  = "%s"
			role_id     = "%s"
			assignments = ["describe", "select"]
		}
	`, project.ID, role.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperProjectRoleAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`(?s)already\s+holds\s+direct\s+grants.*terraform\s+import`),
			},
			{
				Config:             config,
				ResourceName:       "lakekeeper_project_role_assignment.test",
				ImportState:        true,
				ImportStateId:      project.ID + "/" + role.ID,
				ImportStatePersist: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].Attributes["assignments.#"] != "1" || states[0].Attributes["assignments.0"] != "describe" {
						return fmt.Errorf("imported state %v", states)
					}
					return nil
				},
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "id", project.ID+"/"+role.ID),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.#", "2"),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.0", "describe"),
					resource.TestCheckResourceAttr("lakekeeper_project_role_assignment.test", "assignments.1", "select"),
				),
			},
		},
	})
}

func testAccCheckLakekeeperProjectRoleAssignmentDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_project_role_assignment" {
			continue
		}

		projectID, roleID := splitInternalID(types.StringValue(rs.Primary.ID))

		assignments, err := testutil.TestLakekeeperClient.GetProjectAssignments(context.Background(), projectID)
		if err != nil {
			return fmt.Errorf("could not list project assignments to check destroy, %w", err)
		}

		for _, v := range assignments {
			if v.Assignee.ID == roleID && v.Assignee.Type == api.AssigneeTypeRole {
				return fmt.Errorf("project assignment still exists")
			}
		}
	}

	return nil
}
