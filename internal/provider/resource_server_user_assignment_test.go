//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"testing"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperServerUserAssignment_basic(t *testing.T) {

	userID := fmt.Sprintf("oidc~%s", acctest.RandString(32))
	user := testutil.CreateUser(t, userID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperServerUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_server_user_assignment" "test" {
						user_id = "%s"
						assignments = ["operator"]
					}
				`, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "assignments.0", "operator"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_server_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// change one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_server_user_assignment" "test" {
						user_id = "%s"
						assignments = ["admin"]
					}
				`, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "assignments.0", "admin"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_server_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// add one assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_server_user_assignment" "test" {
						user_id = "%s"
						assignments = ["admin", "operator"]
					}
				`, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "assignments.0", "admin"),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "assignments.1", "operator"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_server_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// delete all assignments
			{
				Config: fmt.Sprintf(`				
					resource "lakekeeper_server_user_assignment" "test" {
						user_id = "%s"
						assignments = []
					}
				`, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "user_id", user.ID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "assignments.#", "0"),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_server_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakekeeperServerUserAssignment_userIDWithSlash(t *testing.T) {
	userID := fmt.Sprintf("oidc~%s/%s", acctest.RandString(8), acctest.RandString(8))
	user := testutil.CreateUser(t, userID)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperServerUserAssignmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "lakekeeper_server_user_assignment" "test" {
						user_id = "%s"
						assignments = ["operator"]
					}
				`, user.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "id", userID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "user_id", userID),
					resource.TestCheckResourceAttr("lakekeeper_server_user_assignment.test", "assignments.#", "1"),
				),
			},
			{
				ResourceName:      "lakekeeper_server_user_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckLakekeeperServerUserAssignmentDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_server_user_assignment" {
			continue
		}

		assignments, err := testutil.TestLakekeeperClient.GetServerAssignments(context.Background())
		if err != nil {
			return fmt.Errorf("could not list server assignments to check destroy, %w", err)
		}

		for _, v := range assignments {
			if v.Assignee.ID == rs.Primary.ID && v.Assignee.Type == api.AssigneeTypeUser {
				return fmt.Errorf("server assignment still exists")
			}
		}
	}

	return nil
}
