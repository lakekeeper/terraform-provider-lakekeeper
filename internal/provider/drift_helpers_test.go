//go:build acceptance

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccDeleteOutOfBand returns a check that deletes the object behind a
// resource directly through the API, outside Terraform, by calling del with the
// resource's `id`. Use it in a step with ExpectNonEmptyPlan: true to prove that
// Read drops a vanished object from state and the next plan recreates it.
func testAccDeleteOutOfBand(address string, del func(ctx context.Context, id string) error) func(*terraform.State) error {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("resource %s not found in state", address)
		}
		if err := del(context.Background(), rs.Primary.ID); err != nil {
			return fmt.Errorf("deleting %s outside Terraform: %w", address, err)
		}
		return nil
	}
}
