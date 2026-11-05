//go:build acceptance

package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var testLakekeeperConfig = api.Config{
	BaseURL: os.Getenv("LAKEKEEPER_ENDPOINT"),
	OIDCClientConfig: api.OIDCClientConfig{
		AuthURL:      os.Getenv("LAKEKEEPER_AUTH_URL"),
		ClientID:     os.Getenv("LAKEKEEPER_CLIENT_ID"),
		ClientSecret: os.Getenv("LAKEKEEPER_CLIENT_SECRET"),
		Scopes:       []string{"lakekeeper"},
	},
	InitialBootstrap: true,
}

var TestLakekeeperClient *api.Client

var DefaultUserID string

func init() {
	client, err := testLakekeeperConfig.NewLakekeeperClient(context.Background())
	if err != nil {
		panic("failed to create test client: " + err.Error())
	}

	TestLakekeeperClient = client

	user, err := TestLakekeeperClient.Whoami(context.Background())
	if err != nil {
		panic("failed to get current user: " + err.Error())
	}

	DefaultUserID = user.ID
}

// CreateProject is a test helper for creating a project.
func CreateProject(t *testing.T) *api.Project {
	t.Helper()

	projectID, err := TestLakekeeperClient.CreateProject(t.Context(), acctest.RandomWithPrefix("acctest"))
	if err != nil {
		t.Fatalf("could not create test project: %v", err)
	}

	t.Cleanup(func() {
		if err := TestLakekeeperClient.DeleteProject(context.Background(), projectID); err != nil {
			t.Fatalf("could not cleanup test project: %v", err)
		}
	})

	project, err := TestLakekeeperClient.GetProject(t.Context(), projectID)
	if err != nil {
		t.Fatalf("could not get test project: %v", err)
	}

	return project
}

// CreateWarehouse is a test helper for creating a warehouse.
func CreateWarehouse(t *testing.T, projectID, keyPrefix string) *api.Warehouse {
	t.Helper()

	warehouseID, err := TestLakekeeperClient.CreateWarehouse(t.Context(), projectID, api.CreateWarehouseRequest{
		Name: acctest.RandString(8),
		StorageProfile: &api.S3StorageProfile{
			Bucket:          "testacc",
			Region:          "local-01",
			Endpoint:        new("http://minio:9000/"),
			PathStyleAccess: new(true),
			KeyPrefix:       new(keyPrefix),
		},
		StorageCredential: &api.S3AccessKeyCredential{
			AccessKeyID:     "minio-root-user",
			SecretAccessKey: "minio-root-password",
		},
		DeleteProfile: &api.DeleteProfile{Type: api.DeleteProfileTypeHard},
	})
	if err != nil {
		t.Fatalf("could not create test warehouse: %v", err)
	}

	t.Cleanup(func() {
		if err := TestLakekeeperClient.DeleteWarehouse(context.Background(), projectID, warehouseID, true); err != nil {
			t.Fatalf("could not cleanup test warehouse: %v", err)
		}
	})

	warehouse, err := TestLakekeeperClient.GetWarehouse(t.Context(), projectID, warehouseID)
	if err != nil {
		t.Fatalf("could not create test warehouse: %v", err)
	}

	return warehouse
}

// CreateRole is a test helper for creating a role.
func CreateRole(t *testing.T, projectID string) *api.Role {
	t.Helper()

	role, err := TestLakekeeperClient.CreateRole(t.Context(), projectID, api.RoleRequest{
		Name:        acctest.RandString(8),
		Description: new(acctest.RandString(32)),
	})
	if err != nil {
		t.Fatalf("could not create test role: %v", err)
	}

	t.Cleanup(func() {
		if err := TestLakekeeperClient.DeleteRole(context.Background(), projectID, role.ID); err != nil {
			t.Fatalf("could not cleanup test role: %v", err)
		}
	})

	return role
}

// CreateUser is a test helper for creating a user.
func CreateUser(t *testing.T, id string) *api.User {
	t.Helper()

	name := acctest.RandomWithPrefix("acctest")

	user, err := TestLakekeeperClient.ProvisionUser(t.Context(), api.ProvisionUserRequest{
		ID:       &id,
		Name:     &name,
		Email:    new(fmt.Sprintf("%s@test.com", name)),
		UserType: new(api.UserTypeHuman),
	})
	if err != nil {
		t.Fatalf("could not create test user: %v", err)
	}

	t.Cleanup(func() {
		if err := TestLakekeeperClient.DeleteUser(context.Background(), user.ID); err != nil {
			t.Fatalf("could not cleanup test user: %v", err)
		}
	})

	return user
}
