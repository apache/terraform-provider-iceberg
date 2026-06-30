// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func newTestManagementServer(t *testing.T, mux *http.ServeMux) (*polarisManagementClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(mux)
	client := &polarisManagementClient{
		baseURL:    mustParseURL(srv.URL + "/api/management/v1"),
		httpClient: srv.Client(),
		token:      "test-token",
	}

	return client, srv
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}

	return u
}

// ─── Principal Role Tests ─────────────────────────────────────────────────

func TestCreatePrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var req polarisCreatePrincipalRoleRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "data_engineer", req.PrincipalRole.Name)
		assert.True(t, req.PrincipalRole.Federated)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(polarisPrincipalRole{
			Name:      "data_engineer",
			Federated: true,
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	req := polarisCreatePrincipalRoleRequest{
		PrincipalRole: polarisPrincipalRole{
			Name:      "data_engineer",
			Federated: true,
		},
	}
	result, err := client.CreatePrincipalRole(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "data_engineer", result.Name)
	assert.True(t, result.Federated)
}

func TestCreatePrincipalRole_403(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	_, err := client.CreatePrincipalRole(context.Background(), polarisCreatePrincipalRoleRequest{
		PrincipalRole: polarisPrincipalRole{Name: "test"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestGetPrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/data_engineer", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisPrincipalRole{
			Name:      "data_engineer",
			Federated: false,
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.GetPrincipalRole(ctx, "data_engineer")
	require.NoError(t, err)
	assert.Equal(t, "data_engineer", result.Name)
}

func TestGetPrincipalRole_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	_, err := client.GetPrincipalRole(ctx, "nonexistent")
	require.Error(t, err)
	assert.True(t, isPolarisNotFoundError(err))
}

func TestUpdatePrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/test-role", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req polarisUpdatePrincipalRoleRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, int64(5), req.CurrentEntityVersion)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisPrincipalRole{
			Name:          "test-role",
			EntityVersion: 6,
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.UpdatePrincipalRole(ctx, "test-role", polarisUpdatePrincipalRoleRequest{
		CurrentEntityVersion: 5,
		Properties:           map[string]string{"key": "val"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(6), result.EntityVersion)
}

func TestUpdatePrincipalRole_409(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/test-role", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			http.Error(w, "conflict", http.StatusConflict)
		}
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	_, err := client.UpdatePrincipalRole(ctx, "test-role", polarisUpdatePrincipalRoleRequest{
		CurrentEntityVersion: 5,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "409")
}

func TestDeletePrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/test-role", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.DeletePrincipalRole(ctx, "test-role")
	require.NoError(t, err)
}

func TestDeletePrincipalRole_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.DeletePrincipalRole(ctx, "nonexistent")
	require.Error(t, err)
	assert.True(t, isPolarisNotFoundError(err))
}

func TestListPrincipalRoles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisPrincipalRoles{
			Roles: []polarisPrincipalRole{
				{Name: "role1"},
				{Name: "role2"},
			},
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.ListPrincipalRoles(ctx)
	require.NoError(t, err)
	assert.Len(t, result.Roles, 2)
	assert.Equal(t, "role1", result.Roles[0].Name)
}

// ─── Catalog Role Tests ───────────────────────────────────────────────────

func TestCreateCatalogRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/test-cat/catalog-roles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(polarisCatalogRole{Name: "test-role"})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.CreateCatalogRole(ctx, "test-cat", polarisCreateCatalogRoleRequest{
		CatalogRole: polarisCatalogRole{Name: "test-role"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test-role", result.Name)
}

func TestCreateCatalogRole_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/nonexistent/catalog-roles", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	_, err := client.CreateCatalogRole(ctx, "nonexistent", polarisCreateCatalogRoleRequest{
		CatalogRole: polarisCatalogRole{Name: "test"},
	})
	require.Error(t, err)
	assert.True(t, isPolarisNotFoundError(err))
}

func TestGetCatalogRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/test-cat/catalog-roles/test-role", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisCatalogRole{Name: "test-role", Properties: map[string]string{"k": "v"}})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.GetCatalogRole(ctx, "test-cat", "test-role")
	require.NoError(t, err)
	assert.Equal(t, "test-role", result.Name)
	assert.Equal(t, "v", result.Properties["k"])
}

func TestDeleteCatalogRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/test-cat/catalog-roles/test-role", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.DeleteCatalogRole(ctx, "test-cat", "test-role")
	require.NoError(t, err)
}

func TestListCatalogRoles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/test-cat/catalog-roles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisCatalogRoles{
			Roles: []polarisCatalogRole{
				{Name: "role1", Properties: map[string]string{"a": "b"}},
			},
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.ListCatalogRoles(ctx, "test-cat")
	require.NoError(t, err)
	assert.Len(t, result.Roles, 1)
	assert.Equal(t, "role1", result.Roles[0].Name)
}

// ─── Principal Role Assignment Tests ──────────────────────────────────────

func TestAssignPrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principals/alice/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusCreated)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.AssignPrincipalRole(ctx, "alice", "data_engineer")
	require.NoError(t, err)
}

func TestAssignPrincipalRole_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principals/nonexistent/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.AssignPrincipalRole(ctx, "nonexistent", "test-role")
	require.Error(t, err)
	assert.True(t, isPolarisNotFoundError(err))
}

func TestListPrincipalRoleAssignments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principals/alice/principal-roles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisPrincipalRoles{
			Roles: []polarisPrincipalRole{
				{Name: "data_engineer"},
				{Name: "admin"},
			},
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.ListPrincipalRoleAssignments(ctx, "alice")
	require.NoError(t, err)
	assert.Len(t, result.Roles, 2)
}

func TestRevokePrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principals/alice/principal-roles/data_engineer", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.RevokePrincipalRole(ctx, "alice", "data_engineer")
	require.NoError(t, err)
}

// ─── Catalog Role Assignment Tests ────────────────────────────────────────

func TestAssignCatalogRoleToPrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/data_engineer/catalog-roles/my_catalog", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusCreated)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.AssignCatalogRoleToPrincipalRole(ctx, "data_engineer", "my_catalog", "namespace_admin")
	require.NoError(t, err)
}

func TestListCatalogRoleAssignments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/data_engineer/catalog-roles/my_catalog", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisCatalogRoles{
			Roles: []polarisCatalogRole{
				{Name: "namespace_admin"},
			},
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.ListCatalogRoleAssignments(ctx, "data_engineer", "my_catalog")
	require.NoError(t, err)
	assert.Len(t, result.Roles, 1)
}

func TestRevokeCatalogRoleFromPrincipalRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/principal-roles/data_engineer/catalog-roles/my_catalog/namespace_admin", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.RevokeCatalogRoleFromPrincipalRole(ctx, "data_engineer", "my_catalog", "namespace_admin")
	require.NoError(t, err)
}

// ─── Grant Tests ──────────────────────────────────────────────────────────

func TestAddGrant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/my_catalog/catalog-roles/namespace_admin/grants", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req polarisAddGrantRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "namespace", req.Grant.Type)
		assert.Equal(t, "NAMESPACE_LIST", req.Grant.Privilege)
		w.WriteHeader(http.StatusCreated)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.AddGrant(ctx, "my_catalog", "namespace_admin", polarisGrantResourceBody{
		Type:      "namespace",
		Privilege: "NAMESPACE_LIST",
		Namespace: []string{"my_namespace"},
	})
	require.NoError(t, err)
}

func TestListGrants(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/my_catalog/catalog-roles/namespace_admin/grants", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(polarisGrantResources{
			Grants: []polarisGrantResourceBody{
				{Type: "namespace", Privilege: "NAMESPACE_LIST", Namespace: []string{"my_ns"}},
			},
		})
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	result, err := client.ListGrants(ctx, "my_catalog", "namespace_admin")
	require.NoError(t, err)
	assert.Len(t, result.Grants, 1)
}

func TestRevokeGrant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/management/v1/catalogs/my_catalog/catalog-roles/namespace_admin/grants", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "true", r.URL.Query().Get("cascade"))
		w.WriteHeader(http.StatusCreated)
	})

	client, srv := newTestManagementServer(t, mux)
	defer srv.Close()

	err := client.RevokeGrant(ctx, "my_catalog", "namespace_admin", polarisGrantResourceBody{
		Type:      "namespace",
		Privilege: "NAMESPACE_LIST",
		Namespace: []string{"my_ns"},
	}, true)
	require.NoError(t, err)
}
