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
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperRoleUserAssignment_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	userID := fmt.Sprintf("oidc~%s", acctest.RandString(32))
	user := testutil.CreateUser(t, userID)

	role := testutil.CreateRole(t, project.ID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperRoleUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = "%s"
						user_id = "%s"
						assignments = ["ownership"]
					}
				`, role.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "id", role.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.0", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// change one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = "%s"
						user_id = "%s"
						assignments = ["assignee"]
					}
				`, role.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "id", role.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.#", "1"),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.0", "assignee"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// add one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = "%s"
						user_id = "%s"
						assignments = ["ownership", "assignee"]
					}
				`, role.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "id", role.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.#", "2"),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.0", "assignee"),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.1", "ownership"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Check some basic attribute validation
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = "%s/%s"
						user_id = "%s"
						assignments = ["ownership"]
					}
				`, project.ID, role.ID, user.ID),
				ExpectError: regexp.MustCompile("Attribute role_id must be a UUID"),
			},
			// delete all assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = "%s"
						user_id = "%s"
						assignments = []
					}
				`, role.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "id", role.ID+"/"+user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "role_id", role.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.#", "0"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_role_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakekeeperRoleUserAssignment_userIDWithSlash(t *testing.T) {
	project := testutil.CreateProject(t)
	role := testutil.CreateRole(t, project.ID)

	userID := fmt.Sprintf("oidc~%s/%s", acctest.RandString(8), acctest.RandString(8))
	user := testutil.CreateUser(t, userID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperRoleUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = "%s"
						user_id = "%s"
						assignments = ["assignee"]
					}
				`, role.ID, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "id", role.ID+"/"+userID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "user_id", userID),
					resource.TestCheckResourceAttr("lakekeeper_role_user_assignment.test", "assignments.#", "1"),
				),
			},
			{
				ResourceName:      "lakekeeper_role_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckLakekeeperRoleUserAssignmentDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_role_user_assignment" {
			continue
		}

		parts, err := roleUserAssignment.parseID(rs.Primary.ID)
		if err != nil {
			return err
		}
		roleID, userID := parts[0], parts[1]

		assignments, err := testutil.TestLakekeeperClient.GetRoleAssignments(context.Background(), roleID)
		if err != nil {
			// The role is gone, and its grants with it. Under OpenFGA the
			// assignments of a deleted role answer 403, so ask the role.
			if exists, rerr := testutil.TestLakekeeperClient.RoleExists(context.Background(), roleID); rerr == nil && !exists {
				continue
			}
			return fmt.Errorf("could not list role assignments to check destroy, %w", err)
		}

		for _, v := range assignments {
			if v.Assignee.ID == userID && v.Assignee.Type == api.AssigneeTypeUser {
				return fmt.Errorf("role assignment still exists")
			}
		}
	}

	return nil
}

// When the role is deleted outside Terraform, its grants go with it: the
// assignment leaves state on refresh instead of failing every plan.
func TestAccLakekeeperRoleUserAssignment_scopeDisappears(t *testing.T) {
	project := testutil.CreateProject(t)
	assignee := testutil.CreateUser(t, fmt.Sprintf("oidc~%s", acctest.RandString(32)))
	rName := acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperRoleUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_role" "scope" {
						name = "%s"
						project_id = "%s"
					}
					resource "lakekeeper_role_user_assignment" "test" {
						role_id = lakekeeper_role.scope.role_id
						user_id = "%s"
						assignments = ["ownership"]
					}
				`, rName, project.ID, assignee.ID),
				Check: testAccDeleteOutOfBand("lakekeeper_role.scope", func(ctx context.Context, id string) error {
					projectID, roleID := splitInternalID(types.StringValue(id))
					return testutil.TestLakekeeperClient.DeleteRole(ctx, projectID, roleID)
				}),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks:   testAccExpectAfterRefresh(plancheck.ResourceActionCreate, "lakekeeper_role.scope", "lakekeeper_role_user_assignment.test"),
			},
		},
	})
}
