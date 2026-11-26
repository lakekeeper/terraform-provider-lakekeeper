//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/testutil"
)

func TestAccLakekeeperGenericTableProtection_basic(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))

	namespace := acctest.RandString(8)
	testutil.CreateNamespace(t, project.ID, warehouse, namespace)
	genericTableID := testutil.CreateGenericTable(t, project.ID, warehouse, []string{namespace}, acctest.RandString(8), "dataset")

	const resourceName = "lakekeeper_generic_table_protection.this"

	config := func(protected bool) string {
		return fmt.Sprintf(`
		resource "lakekeeper_generic_table_protection" "this" {
			warehouse_id     = "%s"
			generic_table_id = "%s"
			protected        = %t
		}
		`, warehouse.ID, genericTableID, protected)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLakekeeperGenericTableProtectionDestroy(warehouse.ID, genericTableID),
		Steps: []resource.TestStep{
			{
				Config: config(true),
				Check:  checkProtectionResource(resourceName, "generic_table_id", warehouse.ID, genericTableID, true),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(false),
				Check:  checkProtectionResource(resourceName, "generic_table_id", warehouse.ID, genericTableID, false),
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
					if _, err := testutil.TestLakekeeperClient.SetGenericTableProtection(context.Background(), warehouse.ID, genericTableID, true); err != nil {
						t.Fatalf("could not change generic table protection outside Terraform: %v", err)
					}
				},
				Config: config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: checkProtectionResource(resourceName, "generic_table_id", warehouse.ID, genericTableID, false),
			},
			// Leave the generic table protected so CheckDestroy can prove that
			// destroy does not unprotect it.
			{
				Config: config(true),
				Check:  checkProtectionResource(resourceName, "generic_table_id", warehouse.ID, genericTableID, true),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakekeeperGenericTableProtection_missingGenericTable(t *testing.T) {
	project := testutil.CreateProject(t)
	warehouse := testutil.CreateWarehouse(t, project.ID, fmt.Sprintf("key-prefix-%d", rand.Int()))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "lakekeeper_generic_table_protection" "this" {
					warehouse_id     = "%s"
					generic_table_id = "00000000-0000-0000-0000-000000000000"
					protected        = true
				}
				`, warehouse.ID),
				ExpectError: regexp.MustCompile("may be missing or soft-deleted"),
			},
		},
	})
}

func testAccCheckLakekeeperGenericTableProtectionDestroy(warehouseID, genericTableID string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		protected, err := testutil.TestLakekeeperClient.GetGenericTableProtection(context.Background(), warehouseID, genericTableID)
		if err != nil {
			return fmt.Errorf("could not read generic table protection after destroy: %w", err)
		}
		if !protected {
			return fmt.Errorf("destroy changed the generic table's protection flag; it must leave the server untouched")
		}
		return nil
	}
}
