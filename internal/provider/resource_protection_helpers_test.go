//go:build acceptance

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// checkProtectionResource verifies the attributes shared by the protection
// resources.
func checkProtectionResource(resourceName, objectIDAttribute, warehouseID, objectID string, protected bool) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "id", fmt.Sprintf("%s/%s", warehouseID, objectID)),
		resource.TestCheckResourceAttr(resourceName, "warehouse_id", warehouseID),
		resource.TestCheckResourceAttr(resourceName, objectIDAttribute, objectID),
		resource.TestCheckResourceAttr(resourceName, "protected", fmt.Sprintf("%t", protected)),
	)
}
