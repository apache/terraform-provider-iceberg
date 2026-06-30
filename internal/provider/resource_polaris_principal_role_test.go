package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func providerConfigWithManagementURI(srvAddr string) string {
	return fmt.Sprintf(`
provider "iceberg" {
  type        = "polaris"
  catalog_uri = "%s"

  polaris_settings {
    management_uri = "%s/api/management/v1"
  }
}
`, srvAddr, srvAddr)
}

func newPolarisPrincipalRoleTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	roles := make(map[string]polarisPrincipalRole)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/management/v1/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req polarisCreatePrincipalRoleRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}
			name := req.PrincipalRole.Name
			if name == "" {
				http.Error(w, "missing name", http.StatusBadRequest)

				return
			}
			if _, exists := roles[name]; exists {
				http.Error(w, "already exists", http.StatusConflict)

				return
			}
			role := polarisPrincipalRole{
				Name:       name,
				Federated:  req.PrincipalRole.Federated,
				Properties: req.PrincipalRole.Properties,
			}
			roles[name] = role
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(role)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/management/v1/principal-roles/", func(w http.ResponseWriter, r *http.Request) {
		name := extractLastPathSegment(r.URL.Path)

		switch r.Method {
		case http.MethodGet:
			role, exists := roles[name]
			if !exists {
				http.Error(w, "not found", http.StatusNotFound)

				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(role)

		case http.MethodPut:
			var req polarisUpdatePrincipalRoleRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}
			role, exists := roles[name]
			if !exists {
				http.Error(w, "not found", http.StatusNotFound)

				return
			}
			if req.Properties != nil {
				role.Properties = req.Properties
			}
			role.EntityVersion++
			roles[name] = role
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(role)

		case http.MethodDelete:
			delete(roles, name)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

func extractLastPathSegment(path string) string {
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func TestPrincipalRoleResource_Create(t *testing.T) {
	srv := newPolarisPrincipalRoleTestServer(t)
	defer srv.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfigWithManagementURI(srv.URL) + `
resource "iceberg_polaris_principal_role" "test" {
  name = "test-role"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "name", "test-role"),
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "federated", "false"),
				),
			},
		},
	})
}

func TestPrincipalRoleResource_CreateWithProperties(t *testing.T) {
	srv := newPolarisPrincipalRoleTestServer(t)
	defer srv.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfigWithManagementURI(srv.URL) + `
resource "iceberg_polaris_principal_role" "test" {
  name = "test-role-props"
  properties = {
    team = "platform"
    env  = "prod"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "name", "test-role-props"),
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "properties.team", "platform"),
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "properties.env", "prod"),
				),
			},
		},
	})
}

func TestPrincipalRoleResource_Update(t *testing.T) {
	srv := newPolarisPrincipalRoleTestServer(t)
	defer srv.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfigWithManagementURI(srv.URL) + `
resource "iceberg_polaris_principal_role" "test" {
  name = "test-role-update"
  properties = {
    version = "1"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "properties.version", "1"),
				),
			},
			{
				Config: providerConfigWithManagementURI(srv.URL) + `
resource "iceberg_polaris_principal_role" "test" {
  name = "test-role-update"
  properties = {
    version = "2"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "properties.version", "2"),
				),
			},
		},
	})
}

// TestAccPolarisPrincipalRole_Basic runs against a real Polaris deployment
func TestAccPolarisPrincipalRole_Basic(t *testing.T) {
	catalogURI := os.Getenv("POLARIS_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("POLARIS_CATALOG_URI not set, skipping real-cluster test")
	}
	managementURI := os.Getenv("POLARIS_MANAGEMENT_URI")
	if managementURI == "" {
		managementURI = catalogURI + "/api/management/v1"
	}
	token := os.Getenv("POLARIS_TOKEN")

	providerCfg := testAccPolarisProviderConfigWithToken(catalogURI, managementURI, token)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerCfg + `
resource "iceberg_polaris_principal_role" "test" {
  name = "test-role-acceptance"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_polaris_principal_role.test", "name", "test-role-acceptance"),
				),
			},
		},
	})
}
