// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file implements a small HTTP client for Polaris Management API endpoints
// (e.g. /api/management/v1/...). Iceberg catalog operations use iceberg-go's
// REST catalog client against catalog_uri; this client is only for management
// APIs that are outside the Iceberg REST catalog spec.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

type polarisManagementClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
	headers    map[string]string
}

type polarisNotFoundError struct {
	method string
	path   string
}

func (e *polarisNotFoundError) Error() string {
	return fmt.Sprintf("polaris management: not found %s %s", e.method, e.path)
}

func isPolarisNotFoundError(err error) bool {
	var nf *polarisNotFoundError

	return errors.As(err, &nf)
}

func (p *icebergProvider) newPolarisManagementClient() (*polarisManagementClient, error) {
	if p.polaris == nil || p.polaris.managementURI == "" {
		return nil, errors.New("polaris is not configured: set type = \"polaris\" and ensure polaris_management_uri is set or derivable from catalog_uri")
	}
	u, err := url.Parse(p.polaris.managementURI)
	if err != nil {
		return nil, fmt.Errorf("invalid polaris_management_uri %q: %w", p.polaris.managementURI, err)
	}

	return &polarisManagementClient{
		baseURL:    u,
		httpClient: http.DefaultClient,
		token:      p.token,
		headers:    p.headers,
	}, nil
}

func (c *polarisManagementClient) do(ctx context.Context, method, relativePath string, query url.Values, body any, out any) error {
	u := *c.baseURL

	u.Path = path.Join(c.baseURL.Path, relativePath)
	u.RawQuery = query.Encode()

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	for k, v := range c.headers {
		// don't override existing headers if users are also setting it
		if _, exists := req.Header[k]; !exists {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &polarisNotFoundError{method: method, path: u.Path}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))

		return fmt.Errorf("polaris management: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if out == nil {
		return nil
	}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		if err == io.EOF {
			return nil
		}

		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

type polarisPrincipal struct {
	Name                string            `json:"name"`
	Properties          map[string]string `json:"properties,omitempty"`
	EntityVersion       int64             `json:"entityVersion,omitempty"`
	ClientID            string            `json:"clientId,omitempty"`
	CreateTimestamp     int64             `json:"createTimestamp,omitempty"`
	LastUpdateTimestamp int64             `json:"lastUpdateTimestamp,omitempty"`
}

type polarisPrincipalWithCredentials struct {
	Principal   polarisPrincipal `json:"principal"`
	Credentials struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"credentials"`
}

type polarisCreatePrincipalRequest struct {
	Principal                  polarisPrincipal `json:"principal"`
	CredentialRotationRequired *bool            `json:"credentialRotationRequired,omitempty"`
}

type polarisUpdatePrincipalRequest struct {
	CurrentEntityVersion int64             `json:"currentEntityVersion"`
	Properties           map[string]string `json:"properties,omitempty"`
}

func (c *polarisManagementClient) CreatePrincipal(ctx context.Context, req polarisCreatePrincipalRequest) (*polarisPrincipalWithCredentials, error) {
	var out polarisPrincipalWithCredentials
	if err := c.do(ctx, http.MethodPost, "/principals", nil, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) GetPrincipal(ctx context.Context, name string) (*polarisPrincipal, error) {
	var out polarisPrincipal
	if err := c.do(ctx, http.MethodGet, "/principals/"+url.PathEscape(name), nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) UpdatePrincipal(ctx context.Context, name string, req polarisUpdatePrincipalRequest) (*polarisPrincipal, error) {
	var out polarisPrincipal
	if err := c.do(ctx, http.MethodPut, "/principals/"+url.PathEscape(name), nil, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) DeletePrincipal(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/principals/"+url.PathEscape(name), nil, nil, nil)
}

// ─── Principal Roles ──────────────────────────────────────────────────────

type polarisPrincipalRole struct {
	Name                string            `json:"name"`
	Federated           bool              `json:"federated,omitempty"`
	Properties          map[string]string `json:"properties,omitempty"`
	EntityVersion       int64             `json:"entityVersion,omitempty"`
	CreateTimestamp     int64             `json:"createTimestamp,omitempty"`
	LastUpdateTimestamp int64             `json:"lastUpdateTimestamp,omitempty"`
}

type polarisCreatePrincipalRoleRequest struct {
	PrincipalRole polarisPrincipalRole `json:"principalRole"`
}

type polarisUpdatePrincipalRoleRequest struct {
	CurrentEntityVersion int64             `json:"currentEntityVersion"`
	Properties           map[string]string `json:"properties,omitempty"`
}

type polarisPrincipalRoles struct {
	Roles []polarisPrincipalRole `json:"roles"`
}

func (c *polarisManagementClient) CreatePrincipalRole(ctx context.Context, req polarisCreatePrincipalRoleRequest) (*polarisPrincipalRole, error) {
	var out polarisPrincipalRole
	if err := c.do(ctx, http.MethodPost, "/principal-roles", nil, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) GetPrincipalRole(ctx context.Context, name string) (*polarisPrincipalRole, error) {
	var out polarisPrincipalRole
	if err := c.do(ctx, http.MethodGet, "/principal-roles/"+url.PathEscape(name), nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) UpdatePrincipalRole(ctx context.Context, name string, req polarisUpdatePrincipalRoleRequest) (*polarisPrincipalRole, error) {
	var out polarisPrincipalRole
	if err := c.do(ctx, http.MethodPut, "/principal-roles/"+url.PathEscape(name), nil, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) DeletePrincipalRole(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/principal-roles/"+url.PathEscape(name), nil, nil, nil)
}

func (c *polarisManagementClient) ListPrincipalRoles(ctx context.Context) (*polarisPrincipalRoles, error) {
	var out polarisPrincipalRoles
	if err := c.do(ctx, http.MethodGet, "/principal-roles", nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ─── Principal Role Assignments ───────────────────────────────────────────

type polarisPrincipalRoleAssignment struct {
	PrincipalRoleName string `json:"-"`
}

func (c *polarisManagementClient) AssignPrincipalRole(ctx context.Context, principalName string, roleName string) error {
	body := map[string]any{
		"principalRole": map[string]string{"name": roleName},
	}

	return c.do(ctx, http.MethodPut, "/principals/"+url.PathEscape(principalName)+"/principal-roles", nil, body, nil)
}

func (c *polarisManagementClient) ListPrincipalRoleAssignments(ctx context.Context, principalName string) (*polarisPrincipalRoles, error) {
	var out polarisPrincipalRoles
	if err := c.do(ctx, http.MethodGet, "/principals/"+url.PathEscape(principalName)+"/principal-roles", nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) RevokePrincipalRole(ctx context.Context, principalName string, roleName string) error {
	return c.do(ctx, http.MethodDelete, "/principals/"+url.PathEscape(principalName)+"/principal-roles/"+url.PathEscape(roleName), nil, nil, nil)
}

// ─── Catalog Roles ────────────────────────────────────────────────────────

type polarisCatalogRole struct {
	Name                string            `json:"name"`
	Properties          map[string]string `json:"properties,omitempty"`
	EntityVersion       int64             `json:"entityVersion,omitempty"`
	CreateTimestamp     int64             `json:"createTimestamp,omitempty"`
	LastUpdateTimestamp int64             `json:"lastUpdateTimestamp,omitempty"`
}

type polarisCreateCatalogRoleRequest struct {
	CatalogRole polarisCatalogRole `json:"catalogRole"`
}

type polarisUpdateCatalogRoleRequest struct {
	CurrentEntityVersion int64             `json:"currentEntityVersion"`
	Properties           map[string]string `json:"properties,omitempty"`
}

type polarisCatalogRoles struct {
	Roles []polarisCatalogRole `json:"roles"`
}

func (c *polarisManagementClient) CreateCatalogRole(ctx context.Context, catalogName string, req polarisCreateCatalogRoleRequest) (*polarisCatalogRole, error) {
	var out polarisCatalogRole
	if err := c.do(ctx, http.MethodPost, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles", nil, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) GetCatalogRole(ctx context.Context, catalogName, roleName string) (*polarisCatalogRole, error) {
	var out polarisCatalogRole
	if err := c.do(ctx, http.MethodGet, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles/"+url.PathEscape(roleName), nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) UpdateCatalogRole(ctx context.Context, catalogName, roleName string, req polarisUpdateCatalogRoleRequest) (*polarisCatalogRole, error) {
	var out polarisCatalogRole
	if err := c.do(ctx, http.MethodPut, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles/"+url.PathEscape(roleName), nil, req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) DeleteCatalogRole(ctx context.Context, catalogName, roleName string) error {
	return c.do(ctx, http.MethodDelete, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles/"+url.PathEscape(roleName), nil, nil, nil)
}

func (c *polarisManagementClient) ListCatalogRoles(ctx context.Context, catalogName string) (*polarisCatalogRoles, error) {
	var out polarisCatalogRoles
	if err := c.do(ctx, http.MethodGet, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles", nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ─── Catalog Role Assignments (map catalog role to principal role) ────────

func (c *polarisManagementClient) AssignCatalogRoleToPrincipalRole(ctx context.Context, principalRoleName, catalogName, catalogRoleName string) error {
	body := map[string]any{
		"catalogRole": map[string]string{"name": catalogRoleName},
	}

	return c.do(ctx, http.MethodPut, "/principal-roles/"+url.PathEscape(principalRoleName)+"/catalog-roles/"+url.PathEscape(catalogName), nil, body, nil)
}

func (c *polarisManagementClient) ListCatalogRoleAssignments(ctx context.Context, principalRoleName, catalogName string) (*polarisCatalogRoles, error) {
	var out polarisCatalogRoles
	if err := c.do(ctx, http.MethodGet, "/principal-roles/"+url.PathEscape(principalRoleName)+"/catalog-roles/"+url.PathEscape(catalogName), nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) RevokeCatalogRoleFromPrincipalRole(ctx context.Context, principalRoleName, catalogName, catalogRoleName string) error {
	return c.do(ctx, http.MethodDelete, "/principal-roles/"+url.PathEscape(principalRoleName)+"/catalog-roles/"+url.PathEscape(catalogName)+"/"+url.PathEscape(catalogRoleName), nil, nil, nil)
}

// ─── Grants ───────────────────────────────────────────────────────────────

type polarisGrantResourceBody struct {
	Type      string   `json:"type"`
	Privilege string   `json:"privilege"`
	Namespace []string `json:"namespace,omitempty"`
	TableName string   `json:"tableName,omitempty"`
	ViewName  string   `json:"viewName,omitempty"`
}

type polarisAddGrantRequest struct {
	Grant polarisGrantResourceBody `json:"grant"`
}

type polarisGrantResources struct {
	Grants []polarisGrantResourceBody `json:"grants"`
}

func (c *polarisManagementClient) AddGrant(ctx context.Context, catalogName, catalogRoleName string, grant polarisGrantResourceBody) error {
	body := polarisAddGrantRequest{Grant: grant}

	return c.do(ctx, http.MethodPut, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles/"+url.PathEscape(catalogRoleName)+"/grants", nil, body, nil)
}

func (c *polarisManagementClient) ListGrants(ctx context.Context, catalogName, catalogRoleName string) (*polarisGrantResources, error) {
	var out polarisGrantResources
	if err := c.do(ctx, http.MethodGet, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles/"+url.PathEscape(catalogRoleName)+"/grants", nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *polarisManagementClient) RevokeGrant(ctx context.Context, catalogName, catalogRoleName string, grant polarisGrantResourceBody, cascade bool) error {
	q := url.Values{}
	if cascade {
		q.Set("cascade", "true")
	}
	body := polarisAddGrantRequest{Grant: grant}

	return c.do(ctx, http.MethodPost, "/catalogs/"+url.PathEscape(catalogName)+"/catalog-roles/"+url.PathEscape(catalogRoleName)+"/grants", q, body, nil)
}
