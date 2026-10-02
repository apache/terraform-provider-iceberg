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

package provider

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"regexp"
	"testing"

	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/table"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestIdentifierString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "db.events", identifierString(table.Identifier{"db", "events"}))
	assert.Equal(t, "analytics.raw.orders", identifierString(table.Identifier{"analytics", "raw", "orders"}))
	assert.Equal(t, "", identifierString(nil))
}

func TestSortTableIdentifiers(t *testing.T) {
	t.Parallel()

	ids := []table.Identifier{
		{"analytics", "raw", "orders"},
		{"analytics", "raw", "events"},
		{"ns", "b"},
		{"ns", "a"},
		{"ns", "a"},
	}
	sortTableIdentifiers(ids)

	assert.Equal(t, []table.Identifier{
		{"analytics", "raw", "events"},
		{"analytics", "raw", "orders"},
		{"ns", "a"},
		{"ns", "a"},
		{"ns", "b"},
	}, ids)
}

func TestTableNamesFromIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ids  []table.Identifier
		want []string
	}{
		{
			name: "empty",
			ids:  nil,
			want: []string{},
		},
		{
			name: "single table in flat namespace",
			ids:  []table.Identifier{{"db", "events"}},
			want: []string{"events"},
		},
		{
			name: "preserves caller order",
			ids: []table.Identifier{
				{"analytics", "raw", "orders"},
				{"analytics", "raw", "events"},
			},
			want: []string{"orders", "events"},
		},
		{
			name: "nested namespace",
			ids:  []table.Identifier{{"analytics", "prod", "metrics"}},
			want: []string{"metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tableNamesFromIdentifiers(tt.ids))
		})
	}
}

func TestTableIdentifierStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ids  []table.Identifier
		want []string
	}{
		{
			name: "empty",
			ids:  nil,
			want: []string{},
		},
		{
			name: "single table in flat namespace",
			ids:  []table.Identifier{{"db", "events"}},
			want: []string{"db.events"},
		},
		{
			name: "preserves caller order",
			ids: []table.Identifier{
				{"analytics", "raw", "orders"},
				{"analytics", "raw", "events"},
			},
			want: []string{"analytics.raw.orders", "analytics.raw.events"},
		},
		{
			name: "nested namespace",
			ids:  []table.Identifier{{"analytics", "prod", "metrics"}},
			want: []string{"analytics.prod.metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tableIdentifierStrings(tt.ids))
		})
	}
}

func TestSortedTableListOutputs(t *testing.T) {
	t.Parallel()

	ids := []table.Identifier{
		{"analytics", "raw", "orders"},
		{"analytics", "raw", "events"},
	}
	sortTableIdentifiers(ids)

	assert.Equal(t, []string{"events", "orders"}, tableNamesFromIdentifiers(ids))
	assert.Equal(t, []string{"analytics.raw.events", "analytics.raw.orders"}, tableIdentifierStrings(ids))
}

func listTablesSeq(yields []table.Identifier, finalErr error) iter.Seq2[table.Identifier, error] {
	return func(yield func(table.Identifier, error) bool) {
		for _, ident := range yields {
			if !yield(ident, nil) {
				return
			}
		}
		if finalErr != nil {
			yield(table.Identifier{}, finalErr)
		}
	}
}

type stubNamespaceExister struct {
	exists bool
	err    error
}

func (s stubNamespaceExister) CheckNamespaceExists(context.Context, table.Identifier) (bool, error) {
	return s.exists, s.err
}

type unexpectedNamespaceExister struct {
	t *testing.T
}

func (s unexpectedNamespaceExister) CheckNamespaceExists(context.Context, table.Identifier) (bool, error) {
	s.t.Helper()
	s.t.Fatal("CheckNamespaceExists should not be called")

	return false, nil
}

func TestCollectListedTables(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := table.Identifier{"analytics", "raw"}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		got, err := collectListedTables(ctx, listTablesSeq([]table.Identifier{
			{"analytics", "raw", "orders"},
			{"analytics", "raw", "events"},
		}, nil), ns, unexpectedNamespaceExister{t})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, []table.Identifier{
			{"analytics", "raw", "orders"},
			{"analytics", "raw", "events"},
		}, got)
	})

	t.Run("empty results", func(t *testing.T) {
		t.Parallel()

		got, err := collectListedTables(ctx, listTablesSeq(nil, nil), ns, unexpectedNamespaceExister{t})
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, got)
	})

	t.Run("first yield ErrNoSuchNamespace", func(t *testing.T) {
		t.Parallel()

		got, err := collectListedTables(ctx, listTablesSeq(nil, catalog.ErrNoSuchNamespace), ns, stubNamespaceExister{})
		assert.Nil(t, got)
		if !assert.Error(t, err) {
			return
		}
		assert.ErrorIs(t, err, errNamespaceNotFound)
		assert.Contains(t, err.Error(), "analytics.raw")
		// Catalog sentinel is intentionally not wrapped here; Read keys off
		// errNamespaceNotFound so mid-pagination ErrNoSuchNamespace stays distinct.
		assert.False(t, errors.Is(err, catalog.ErrNoSuchNamespace))
	})

	t.Run("generic error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("catalog unavailable")
		got, err := collectListedTables(ctx, listTablesSeq(nil, wantErr), ns, unexpectedNamespaceExister{t})
		assert.Nil(t, got)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("error after partial results preserves cause", func(t *testing.T) {
		t.Parallel()

		// Mimics iceberg-go REST pagination: identifiers from page 1, then a
		// page-level HTTP 404 wrapped as ErrNoSuchNamespace (e.g. bad page token).
		pageErr := fmt.Errorf("NoSuchPageTokenException: %w", catalog.ErrNoSuchNamespace)
		got, err := collectListedTables(ctx, listTablesSeq([]table.Identifier{
			{"analytics", "raw", "events"},
			{"analytics", "raw", "orders"},
		}, pageErr), ns, stubNamespaceExister{exists: true})
		assert.Nil(t, got)
		if !assert.Error(t, err) {
			return
		}
		assert.ErrorIs(t, err, catalog.ErrNoSuchNamespace)
		assert.False(t, errors.Is(err, errNamespaceNotFound))
		assert.Contains(t, err.Error(), "NoSuchPageTokenException")
	})

	t.Run("empty first page then pagination 404 is not missing namespace", func(t *testing.T) {
		t.Parallel()

		// Empty non-final page yields nothing, then a later page 404. Yield
		// count is still 0, but CheckNamespaceExists reports the namespace
		// is present — must not become errNamespaceNotFound.
		pageErr := fmt.Errorf("NoSuchPageTokenException: %w", catalog.ErrNoSuchNamespace)
		got, err := collectListedTables(ctx, listTablesSeq(nil, pageErr), ns, stubNamespaceExister{exists: true})
		assert.Nil(t, got)
		if !assert.Error(t, err) {
			return
		}
		assert.ErrorIs(t, err, catalog.ErrNoSuchNamespace)
		assert.False(t, errors.Is(err, errNamespaceNotFound))
		assert.Contains(t, err.Error(), "NoSuchPageTokenException")
	})

	t.Run("existence probe error preserves list error", func(t *testing.T) {
		t.Parallel()

		pageErr := fmt.Errorf("NoSuchPageTokenException: %w", catalog.ErrNoSuchNamespace)
		got, err := collectListedTables(ctx, listTablesSeq(nil, pageErr), ns, stubNamespaceExister{
			err: errors.New("HEAD failed"),
		})
		assert.Nil(t, got)
		if !assert.Error(t, err) {
			return
		}
		assert.ErrorIs(t, err, catalog.ErrNoSuchNamespace)
		assert.False(t, errors.Is(err, errNamespaceNotFound))
		assert.Contains(t, err.Error(), "NoSuchPageTokenException")
	})

	t.Run("rejects empty identifier", func(t *testing.T) {
		t.Parallel()

		got, err := collectListedTables(ctx, listTablesSeq([]table.Identifier{{}}, nil), ns, unexpectedNamespaceExister{t})
		assert.Nil(t, got)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "empty table identifier")
	})

	t.Run("rejects identifier outside namespace", func(t *testing.T) {
		t.Parallel()

		got, err := collectListedTables(ctx, listTablesSeq([]table.Identifier{
			{"other", "ns", "events"},
		}, nil), ns, unexpectedNamespaceExister{t})
		assert.Nil(t, got)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), `outside requested namespace "analytics.raw"`)
	})
}

func TestAccIcebergTablesDataSource_Full(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("ICEBERG_CATALOG_URI not set, skipping tables data source E2E test")
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	suffix := acctest.RandString(8)
	basicNS := "ns_tables_ds_basic_" + suffix
	nestedParent := "analytics_" + suffix
	emptyNS := "ns_tables_ds_empty_" + suffix

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTablesDataSourceBasicConfig(providerCfg, basicNS),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "namespace.0", basicNS),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "id", basicNS),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "tables.#", "2"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "tables.0", "alpha_table"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "tables.1", "beta_table"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "identifiers.#", "2"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "identifiers.0", basicNS+".alpha_table"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "identifiers.1", basicNS+".beta_table"),
				),
			},
			{
				Config: testAccIcebergTablesDataSourceNestedConfig(providerCfg, nestedParent),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "namespace.0", nestedParent),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "namespace.1", "raw"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "id", nestedParent+".raw"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "tables.#", "1"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "tables.0", "events"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "identifiers.#", "1"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "identifiers.0", nestedParent+".raw.events"),
				),
			},
			{
				Config: testAccIcebergTablesDataSourceEmptyNamespaceConfig(providerCfg, emptyNS),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "namespace.0", emptyNS),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "tables.#", "0"),
					resource.TestCheckResourceAttr("data.iceberg_tables.read", "identifiers.#", "0"),
				),
			},
		},
	})
}

func TestAccIcebergTablesDataSource_NotFound(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("ICEBERG_CATALOG_URI not set, skipping tables data source E2E test")
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	missingNS := "ns_tables_ds_missing_" + acctest.RandString(8)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccIcebergTablesDataSourceMissingConfig(providerCfg, missingNS),
				ExpectError: regexp.MustCompile(`Namespace not found`),
			},
		},
	})
}

func TestAccIcebergTablesDataSource_EmptyNamespaceAttribute(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("ICEBERG_CATALOG_URI not set, skipping tables data source E2E test")
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccIcebergTablesDataSourceEmptyNamespaceAttributeConfig(providerCfg),
				ExpectError: regexp.MustCompile(`(?s)Invalid namespace.*at least one namespace segment`),
			},
		},
	})
}

func testAccIcebergTablesDataSourceBasicConfig(providerCfg, namespace string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db" {
  name = [%q]
}

resource "iceberg_table" "alpha" {
  namespace = iceberg_namespace.db.name
  name      = "alpha_table"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
}

resource "iceberg_table" "beta" {
  namespace = iceberg_namespace.db.name
  name      = "beta_table"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
}

data "iceberg_tables" "read" {
  namespace = iceberg_namespace.db.name

  depends_on = [
    iceberg_table.alpha,
    iceberg_table.beta,
  ]
}
`, namespace)
}

func testAccIcebergTablesDataSourceNestedConfig(providerCfg, parentNS string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db" {
  name = [%q, "raw"]
}

resource "iceberg_table" "events" {
  namespace = iceberg_namespace.db.name
  name      = "events"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
}

data "iceberg_tables" "read" {
  namespace = iceberg_namespace.db.name

  depends_on = [iceberg_table.events]
}
`, parentNS)
}

func testAccIcebergTablesDataSourceEmptyNamespaceConfig(providerCfg, namespace string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db" {
  name = [%q]
}

data "iceberg_tables" "read" {
  namespace = iceberg_namespace.db.name
}
`, namespace)
}

func testAccIcebergTablesDataSourceEmptyNamespaceAttributeConfig(providerCfg string) string {
	return providerCfg + `
data "iceberg_tables" "empty_attr" {
  namespace = []
}
`
}

func testAccIcebergTablesDataSourceMissingConfig(providerCfg, namespace string) string {
	return providerCfg + fmt.Sprintf(`
data "iceberg_tables" "missing" {
  namespace = [%q]
}
`, namespace)
}
