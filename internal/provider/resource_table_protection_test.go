//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"testing"

	"github.com/apache/iceberg-go/catalog"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccLakekeeperTableProtection_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))

	namespace := acctest.RandString(8)
	testutil.CreateNamespace(t, project.ID, warehouse, namespace)
	tableID := testutil.CreateTable(t, project.ID, warehouse, namespace, acctest.RandString(8))

	const resourceName = "lakekeeper_table_protection.this"

	config := func(protected bool) string {
		return fmt.Sprintf(`
		resource "lakekeeper_table_protection" "this" {
			warehouse_id = "%s"
			table_id     = "%s"
			protected    = %t
		}
		`, warehouse.ID, tableID, protected)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperTableProtectionDestroy(warehouse.ID, tableID),
		Steps: []resource.TestStep{
			{
				Config: config(true),
				Check:  checkProtectionResource(resourceName, "table_id", warehouse.ID, tableID, true),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(false),
				Check:  checkProtectionResource(resourceName, "table_id", warehouse.ID, tableID, false),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Change the flag outside Terraform: the refresh must detect the
				// drift and plan an update back to the configured value.
				PreConfig: func() {
					if _, err := testutil.TestLakekeeperClient.SetTableProtection(context.Background(), warehouse.ID, tableID, true); err != nil {
						t.Fatalf("could not change table protection outside Terraform: %v", err)
					}
				},
				Config: config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: checkProtectionResource(resourceName, "table_id", warehouse.ID, tableID, false),
			},
			// Leave the table protected so CheckDestroy can prove that destroy
			// does not unprotect it.
			{
				Config: config(true),
				Check:  checkProtectionResource(resourceName, "table_id", warehouse.ID, tableID, true),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// A table dropped outside Terraform leaves state on refresh, so the next plan
// tries to create the protection resource again instead of failing on a stale
// table ID.
func TestAccLakekeeperTableProtection_disappears(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))

	namespace := acctest.RandString(8)
	testutil.CreateNamespace(t, project.ID, warehouse, namespace)
	tableName := acctest.RandString(8)
	tableID := testutil.CreateTable(t, project.ID, warehouse, namespace, tableName)

	const resourceName = "lakekeeper_table_protection.this"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "lakekeeper_table_protection" "this" {
					warehouse_id = "%s"
					table_id     = "%s"
					protected    = true
				}
				`, warehouse.ID, tableID),
				Check: checkProtectionResource(resourceName, "table_id", warehouse.ID, tableID, true),
			},
			{
				// Drop the table outside Terraform. The refresh must forget the
				// protection resource, and the followup plan must want to
				// create it again.
				PreConfig: func() {
					ctx := context.Background()
					if _, err := testutil.TestLakekeeperClient.SetTableProtection(ctx, warehouse.ID, tableID, false); err != nil {
						t.Fatalf("could not unprotect table outside Terraform: %v", err)
					}
					cat, err := testutil.TestLakekeeperClient.Catalog(ctx, project.ID, warehouse.Name)
					if err != nil {
						t.Fatalf("could not create catalog client: %v", err)
					}
					if err := cat.DropTable(ctx, catalog.ToIdentifier(namespace, tableName)); err != nil {
						t.Fatalf("could not drop table outside Terraform: %v", err)
					}
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
				Check: func(s *terraform.State) error {
					if _, ok := s.RootModule().Resources[resourceName]; ok {
						return fmt.Errorf("%s is still in state after its table was dropped outside Terraform", resourceName)
					}
					return nil
				},
			},
		},
	})
}

func TestAccLakekeeperTableProtection_missingTable(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "lakekeeper_table_protection" "this" {
					warehouse_id = "%s"
					table_id     = "00000000-0000-0000-0000-000000000000"
					protected    = true
				}
				`, warehouse.ID),
				ExpectError: regexp.MustCompile("may be missing or soft-deleted"),
			},
		},
	})
}

func testAccCheckLakekeeperTableProtectionDestroy(warehouseID, tableID string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		protected, err := testutil.TestLakekeeperClient.GetTableProtection(context.Background(), warehouseID, tableID)
		if err != nil {
			return fmt.Errorf("could not read table protection after destroy: %w", err)
		}
		if !protected {
			return fmt.Errorf("destroy changed the table's protection flag; it must leave the server untouched")
		}
		return nil
	}
}
