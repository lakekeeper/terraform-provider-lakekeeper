//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/apache/iceberg-go/catalog"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLakekeeperNamespace_basic(t *testing.T) {

	project := testutil.CreateProject(t)

	keyPrefix := fmt.Sprintf("key-prefix-%d", rand.Int())
	warehouse := testutil.CreateWarehouse(t, project.ID, keyPrefix)

	rName := acctest.RandString(8)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperNamespaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "lakekeeper_namespace" "this" {
					project_id = "%s"
					warehouse_name = "%s"
					name = "%s"
				}
				`, project.ID, warehouse.Name, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_namespace.this", "id", fmt.Sprintf("%s/%s/%s", project.ID, warehouse.Name, rName)),
					resource.TestCheckResourceAttr("lakekeeper_namespace.this", "project_id", project.ID),
					resource.TestCheckResourceAttr("lakekeeper_namespace.this", "warehouse_name", warehouse.Name),
					resource.TestCheckResourceAttr("lakekeeper_namespace.this", "name", rName),
				),
			},
			// Verify import
			{
				ResourceName:      "lakekeeper_namespace.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakekeeperNamespace_adoptExisting(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))
	rName := acctest.RandString(8)

	// The namespace exists before Terraform creates it, as after it drops out of state.
	cat, err := testutil.TestLakekeeperClient.Catalog(t.Context(), project.ID, warehouse.Name)
	if err != nil {
		t.Fatalf("could not create the Iceberg Catalog client: %v", err)
	}
	if err := cat.CreateNamespace(t.Context(), catalog.ToIdentifier(rName), nil); err != nil {
		t.Fatalf("could not create namespace %s: %v", rName, err)
	}
	props, err := cat.LoadNamespaceProperties(t.Context(), catalog.ToIdentifier(rName))
	if err != nil {
		t.Fatalf("could not load namespace %s: %v", rName, err)
	}

	config := func(adopt string) string {
		return fmt.Sprintf(`
		resource "lakekeeper_namespace" "this" {
			project_id     = "%s"
			warehouse_name = "%s"
			name           = "%s"
			%s
		}
		`, project.ID, warehouse.Name, rName, adopt)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperNamespaceDestroy,
		Steps: []resource.TestStep{
			{
				Config:      config(""),
				ExpectError: regexp.MustCompile(`already exist`),
			},
			{
				Config: config("adopt_existing = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lakekeeper_namespace.this", "namespace_id", props["namespace_id"]),
					resource.TestCheckResourceAttr("lakekeeper_namespace.this", "adopt_existing", "true"),
				),
			},
			{
				ResourceName:            "lakekeeper_namespace.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"adopt_existing"},
			},
		},
	})
}

func testAccCheckLakekeeperNamespaceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "lakekeeper_namespace" {
			continue
		}

		splitted := strings.Split(rs.Primary.ID, "/")
		projectID := splitted[0]
		warehouseName := splitted[1]
		name := splitted[2]

		ctx := context.Background()

		cat, err := testutil.TestLakekeeperClient.Catalog(ctx, projectID, warehouseName)
		if err != nil {
			return fmt.Errorf("could not create the Iceberg Catalog client, %w", err)
		}

		exists, err := cat.CheckNamespaceExists(ctx, catalog.ToIdentifier(name))
		if err != nil {
			return fmt.Errorf("could not check if namespace exists, %w", err)
		}
		if exists {
			return fmt.Errorf("namespace still exists")
		}
		return nil
	}
	return nil
}
