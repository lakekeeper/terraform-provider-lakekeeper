//go:build acceptance

package provider

import (
	"fmt"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccDataLakekeeperProjectRoleAccess_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	role := testutil.CreateRole(t, project.ID)

	// assignment
	if err := testutil.TestLakekeeperClient.UpdateProjectAssignments(
		t.Context(),
		project.ID,
		[]api.Assignment{
			{
				Assignee:   api.RoleAssignee(role.ID),
				Assignment: api.ProjectAssignmentAdmin,
			},
		},
		nil,
	); err != nil {
		t.Fatalf("could not create project access, %v", err)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				data "lakekeeper_project_role_access" "foo" {
					project_id = "%s"
					role_id = "%s"
				}`, project.ID, role.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.lakekeeper_project_role_access.foo", "id", fmt.Sprintf("%s/%s", project.ID, role.ID)),
					resource.TestCheckResourceAttr("data.lakekeeper_project_role_access.foo", "project_id", project.ID),
					resource.TestCheckResourceAttr("data.lakekeeper_project_role_access.foo", "role_id", role.ID),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.lakekeeper_project_role_access.foo",
						tfjsonpath.Path(
							tfjsonpath.New("allowed_actions"),
						),
						knownvalue.SetPartial([]knownvalue.Check{
							knownvalue.StringFunc(func(v string) error {
								if !slices.Contains([]string{
									api.ProjectActionCreateRole,
									api.ProjectActionCreateWarehouse,
									api.ProjectActionDelete,
									api.ProjectActionRename,
									api.ProjectActionListWarehouses,
									api.ProjectActionListRoles,
									api.ProjectActionSearchRoles,
									api.ProjectActionReadAssignments,
									api.ProjectActionGrantRoleCreator,
									api.ProjectActionGrantCreate,
									api.ProjectActionGrantDescribe,
									api.ProjectActionGrantModify,
									api.ProjectActionGrantSelect,
									api.ProjectActionGrantProjectAdmin,
									api.ProjectActionGrantSecurityAdmin,
									api.ProjectActionGrantDataAdmin,
									api.ProjectActionGetEndpointStatistics,
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
