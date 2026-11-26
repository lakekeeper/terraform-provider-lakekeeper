//go:build acceptance

package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/view"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// testTableSchema is a minimal schema that is valid for tables and views.
var testTableSchema = iceberg.NewSchemaWithIdentifiers(1, []int{1},
	iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
	iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String},
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
			Endpoint:        new("http://objectstore:9000/"),
			PathStyleAccess: new(true),
			KeyPrefix:       new(keyPrefix),
		},
		StorageCredential: &api.S3AccessKeyCredential{
			AccessKeyID:     "testacc-access-key",
			SecretAccessKey: "testacc-secret-key",
		},
		DeleteProfile: &api.DeleteProfile{Type: api.DeleteProfileTypeHard},
	})
	if err != nil {
		t.Fatalf("could not create test warehouse: %v", err)
	}

	t.Cleanup(func() {
		// Dropping tables, views and generic tables queues purge tasks. A
		// warehouse cannot be deleted until those finish, so retry briefly
		// instead of racing the task worker.
		ctx := context.Background()
		var err error
		for attempt := 0; attempt < 30; attempt++ {
			if err = TestLakekeeperClient.DeleteWarehouse(ctx, projectID, warehouseID, true); err == nil {
				return
			}
			if !strings.Contains(err.Error(), "WarehouseHasUnfinishedTasks") {
				break
			}
			time.Sleep(time.Second)
		}
		t.Fatalf("could not cleanup test warehouse: %v", err)
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

// CreateNamespace is a test helper for creating a namespace. Cleanup drops it
// before the warehouse is removed. Pass several levels to create a nested
// namespace.
func CreateNamespace(t *testing.T, projectID string, warehouse *api.Warehouse, path ...string) {
	t.Helper()

	cat, err := TestLakekeeperClient.Catalog(t.Context(), projectID, warehouse.Name)
	if err != nil {
		t.Fatalf("could not create test catalog client: %v", err)
	}

	if err := cat.CreateNamespace(t.Context(), catalog.ToIdentifier(path...), nil); err != nil {
		t.Fatalf("could not create test namespace: %v", err)
	}

	t.Cleanup(func() {
		if err := cat.DropNamespace(context.Background(), catalog.ToIdentifier(path...)); err != nil {
			t.Errorf("could not cleanup test namespace: %v", err)
		}
	})
}

// CreateTable is a test helper for creating an Iceberg table. It returns the
// table's UUID, which the management API uses. Cleanup clears protection and
// drops the table.
func CreateTable(t *testing.T, projectID string, warehouse *api.Warehouse, namespace, name string) string {
	t.Helper()

	cat, err := TestLakekeeperClient.Catalog(t.Context(), projectID, warehouse.Name)
	if err != nil {
		t.Fatalf("could not create test catalog client: %v", err)
	}

	ident := catalog.ToIdentifier(namespace, name)
	tbl, err := cat.CreateTable(t.Context(), ident, testTableSchema)
	if err != nil {
		t.Fatalf("could not create test table: %v", err)
	}
	id := tbl.Metadata().TableUUID().String()

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := TestLakekeeperClient.SetTableProtection(ctx, warehouse.ID, id, false); err != nil {
			if api.IsNotFound(err) {
				// The test dropped the table outside Terraform.
				return
			}
			t.Errorf("could not unprotect test table: %v", err)
		}
		if err := cat.DropTable(ctx, ident); err != nil {
			t.Errorf("could not cleanup test table: %v", err)
		}
	})

	return id
}

// CreateView is a test helper for creating a view. It returns the view's UUID,
// which the management API uses. Cleanup clears protection and drops the view.
func CreateView(t *testing.T, projectID string, warehouse *api.Warehouse, namespace, name string) string {
	t.Helper()

	cat, err := TestLakekeeperClient.Catalog(t.Context(), projectID, warehouse.Name)
	if err != nil {
		t.Fatalf("could not create test catalog client: %v", err)
	}

	version, err := view.NewVersion(1, 1,
		[]view.Representation{view.NewRepresentation("SELECT 1", "trino")},
		catalog.ToIdentifier(namespace))
	if err != nil {
		t.Fatalf("could not build test view version: %v", err)
	}

	ident := catalog.ToIdentifier(namespace, name)
	v, err := cat.CreateView(t.Context(), ident, version, testTableSchema)
	if err != nil {
		t.Fatalf("could not create test view: %v", err)
	}
	id := v.Metadata().ViewUUID().String()

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := TestLakekeeperClient.SetViewProtection(ctx, warehouse.ID, id, false); err != nil {
			t.Errorf("could not unprotect test view: %v", err)
		}
		if err := cat.DropView(ctx, ident); err != nil {
			t.Errorf("could not cleanup test view: %v", err)
		}
	})

	return id
}

// CreateGenericTable is a test helper for creating a generic table. It returns
// the generic table's UUID, which the management API uses. Cleanup force-drops
// it, which also works while it is protected.
func CreateGenericTable(t *testing.T, projectID string, warehouse *api.Warehouse, namespace []string, name, format string) string {
	t.Helper()

	id, err := TestLakekeeperClient.CreateGenericTable(t.Context(), projectID, warehouse.ID, namespace, api.GenericTableRequest{Name: name, Format: format})
	if err != nil {
		t.Fatalf("could not create test generic table: %v", err)
	}

	t.Cleanup(func() {
		if err := TestLakekeeperClient.DeleteGenericTable(context.Background(), projectID, warehouse.ID, namespace, name, false, true); err != nil {
			t.Errorf("could not cleanup test generic table: %v", err)
		}
	})

	return id.ID
}
