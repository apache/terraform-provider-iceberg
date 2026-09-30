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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestProvider(t *testing.T) {
	assert.NotNil(t, New()())
}

// newOAuth2CatalogServer returns a fake REST catalog that issues a token from
// tokenPath and rejects catalog requests that don't carry exactly that token
// and the wantHeaders.
func newOAuth2CatalogServer(t *testing.T, tokenPath string, wantForm, wantHeaders map[string]string) *httptest.Server {
	t.Helper()

	const token = "issued-token"
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+tokenPath, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}
		for k, v := range wantForm {
			if got := r.PostForm.Get(k); got != v {
				http.Error(w, fmt.Sprintf("form %s: got %q, want %q", k, got, v), http.StatusBadRequest)

				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Values("Authorization"); len(auth) != 1 || auth[0] != "Bearer "+token {
			http.Error(w, `{"error":{"message":"unauthorized","type":"NotAuthorizedException","code":401}}`, http.StatusUnauthorized)

			return
		}
		for k, v := range wantHeaders {
			if got := r.Header.Values(k); len(got) != 1 || got[0] != v {
				http.Error(w, fmt.Sprintf(`{"error":{"message":"header %s: got %q, want %q","type":"BadRequestException","code":400}}`, k, got, v), http.StatusBadRequest)

				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/config":
			_, _ = w.Write([]byte(`{"defaults":{},"overrides":{}}`))
		case "/v1/namespaces/ns":
			_, _ = w.Write([]byte(`{"namespace":["ns"],"properties":{"owner":"oauth"}}`))
		default:
			http.NotFound(w, r)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func TestOAuth2ClientCredentials(t *testing.T) {
	srv := newOAuth2CatalogServer(t, "/v1/oauth/tokens", map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     "client",
		"client_secret": "secret",
		"scope":         "catalog",
	}, nil)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "iceberg" {
  catalog_uri = %q
  auth = {
    oauth2 = {
      credential = "client:secret"
    }
  }
}

data "iceberg_namespace" "ns" {
  name = ["ns"]
}
`, srv.URL),
				Check: resource.TestCheckResourceAttr("data.iceberg_namespace.ns", "server_properties.owner", "oauth"),
			},
		},
	})
}

func TestOAuth2CustomServerURI(t *testing.T) {
	srv := newOAuth2CatalogServer(t, "/idp/token", map[string]string{
		"client_id":     "client",
		"client_secret": "secret",
		"scope":         "PRINCIPAL_ROLE:ALL",
		"audience":      "my-audience",
		"resource":      "my-resource",
	}, nil)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "iceberg" {
  catalog_uri = %[1]q
  auth = {
    oauth2 = {
      credential = "client:secret"
      server_uri = "%[1]s/idp/token"
      scope      = "PRINCIPAL_ROLE:ALL"
      audience   = "my-audience"
      resource   = "my-resource"
    }
  }
}

data "iceberg_namespace" "ns" {
  name = ["ns"]
}
`, srv.URL),
				Check: resource.TestCheckResourceAttr("data.iceberg_namespace.ns", "server_properties.owner", "oauth"),
			},
		},
	})
}

func TestOAuth2WithHeaders(t *testing.T) {
	srv := newOAuth2CatalogServer(t, "/v1/oauth/tokens", nil, map[string]string{
		"X-Custom": "value",
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The OAuth2 token must replace, not duplicate, a configured
				// Authorization header.
				Config: fmt.Sprintf(`
provider "iceberg" {
  catalog_uri = %q
  headers = {
    Authorization = "Bearer stale"
    X-Custom      = "value"
  }
  auth = {
    oauth2 = {
      credential = "client:secret"
    }
  }
}

data "iceberg_namespace" "ns" {
  name = ["ns"]
}
`, srv.URL),
				Check: resource.TestCheckResourceAttr("data.iceberg_namespace.ns", "server_properties.owner", "oauth"),
			},
		},
	})
}

func TestOAuth2Validation(t *testing.T) {
	cases := map[string]struct {
		auth    string
		wantErr string
	}{
		"conflicts with token": {
			auth: `token = "static"
  auth = { oauth2 = { credential = "client:secret" } }`,
			wantErr: `(?s)Invalid Attribute Combination`,
		},
		"relative server_uri": {
			auth:    `auth = { oauth2 = { credential = "client:secret", server_uri = "/oauth/tokens" } }`,
			wantErr: `Invalid OAuth2 Server URI`,
		},
		"missing credential": {
			auth:    `auth = { oauth2 = { scope = "catalog" } }`,
			wantErr: `(?s)attribute\s+"credential" is required`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: fmt.Sprintf(`
provider "iceberg" {
  catalog_uri = "http://localhost:1"
  %s
}

data "iceberg_namespace" "ns" {
  name = ["ns"]
}
`, tc.auth),
						ExpectError: regexp.MustCompile(tc.wantErr),
					},
				},
			})
		})
	}
}
