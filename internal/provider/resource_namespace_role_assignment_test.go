//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"testing"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperNamespaceRoleAssignment_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))
	role := testutil.CreateRole(t, project.ID)
	name := acctest.RandString(8)
	address := "lakekeeper_namespace_role_assignment.test"

	namespace := fmt.Sprintf(`
		resource "lakekeeper_namespace" "test" {
			project_id     = "%s"
			warehouse_name = "%s"
			name           = "%s"
		}
	`, project.ID, warehouse.Name, name)
	config := func(roleID, grants string) string {
		return namespace + fmt.Sprintf(`
			resource "lakekeeper_namespace_role_assignment" "test" {
				project_id   = "%s"
				namespace_id = lakekeeper_namespace.test.namespace_id
				role_id      = "%s"
				assignments  = [%s]
			}
		`, project.ID, roleID, grants)
	}
	checkGrants := func(grants ...string) resource.TestCheckFunc {
		checks := []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "project_id", project.ID),
			resource.TestCheckResourceAttr(address, "role_id", role.ID),
			resource.TestCheckResourceAttrPair(address, "namespace_id", "lakekeeper_namespace.test", "namespace_id"),
			resource.TestMatchResourceAttr(address, "id", regexp.MustCompile("^"+project.ID+"/[0-9a-f-]{36}/"+role.ID+"$")),
			resource.TestCheckResourceAttr(address, "assignments.#", fmt.Sprint(len(grants))),
		}
		for _, g := range grants {
			checks = append(checks, resource.TestCheckTypeSetElemAttr(address, "assignments.*", g))
		}
		return resource.ComposeAggregateTestCheckFunc(checks...)
	}
	importStep := resource.TestStep{ResourceName: address, ImportState: true, ImportStateVerify: true}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(role.ID, `"select"`), Check: checkGrants("select")},
			importStep,
			// Revokes select and grants describe and modify in one request.
			{Config: config(role.ID, `"describe", "modify"`), Check: checkGrants("describe", "modify")},
			importStep,
			{Config: config(project.ID+"/"+role.ID, `"describe"`), ExpectError: regexp.MustCompile("Attribute role_id must be a UUID")},
			{Config: config(role.ID, ``), Check: checkGrants()},
			importStep,
			// Removing the resource revokes its grants; the namespace stays so they can be listed.
			{Config: config(role.ID, `"select"`), Check: checkGrants("select")},
			{Config: namespace, Check: testAccCheckNamespaceRoleGrantsRevoked(project.ID, role.ID)},
		},
	})
}

// testAccCheckNamespaceRoleGrantsRevoked checks that the role holds no direct
// grant on the namespace in state.
func testAccCheckNamespaceRoleGrantsRevoked(projectID, roleID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ns, ok := s.RootModule().Resources["lakekeeper_namespace.test"]
		if !ok {
			return fmt.Errorf("namespace not in state")
		}
		scope := assignmentScope{"project_id": projectID, "namespace_id": ns.Primary.Attributes["namespace_id"]}
		grants, err := newNamespaceScopeAdapter(testutil.TestLakekeeperClient).ListGrants(context.Background(), scope)
		if err != nil {
			return fmt.Errorf("could not list namespace grants: %w", err)
		}
		for _, g := range grants {
			if g.AssigneeKind == roleAssignee && g.AssigneeID == roleID {
				return fmt.Errorf("namespace grant %q of the role still exists", g.Type)
			}
		}
		return nil
	}
}
