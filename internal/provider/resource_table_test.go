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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/catalog/rest"
	"github.com/apache/iceberg-go/table"
	"github.com/apache/iceberg-go/view"
	"github.com/hashicorp/terraform-plugin-framework/path"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
)

func TestAccIcebergTable(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableResourceConfig(providerCfg, "test_table"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "namespace.0", "db1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "test_table"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.type", "long"),
				),
			},
			{
				ResourceName:      "iceberg_table.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"server_properties",
				},
			},
		},
	})
}

func testAccIcebergTableResourceConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db1" {
  name = ["db1"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db1.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      },
      {
        id       = 2
        name     = "data"
        type     = "string"
        required = false
      }
    ]
  }
}
`, tableName)
}

func testAccIcebergTableUpdateConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db1" {
  name = ["db1"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db1.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      },
      {
        id       = 2
        name     = "data"
        type     = "string"
        required = false
      },
      {
        id       = 3
        name     = "new_field"
        type     = "int"
        required = false
      }
    ]
  }
}
`, tableName)
}

func TestAccIcebergTableUpdate(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "update_test_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableResourceConfig(providerCfg, tableName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", tableName),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.#", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
				),
			},
			{
				Config: testAccIcebergTableUpdateConfig(providerCfg, tableName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", tableName),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.#", "3"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.name", "new_field"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.type", "int"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "1"),
				),
			},
		},
	})
}

func testAccIcebergTableOmittedIDsConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_omit" {
  name = ["db_omit"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_omit.name
  name      = "%s"
  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      },
      {
        name     = "data"
        type     = "string"
        required = false
      },
      {
        name     = "tags"
        type     = "list"
        required = false
        list_properties = {
          element_type     = "string"
          element_required = true
        }
      }
    ]
  }
}
`, tableName)
}

func TestAccIcebergTableOmittedFieldIDs(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "omitted_ids_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableOmittedIDsConfig(providerCfg, tableName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.#", "3"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.name", "data"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.name", "tags"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.id", "3"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.list_properties.element_id", "4"),
				),
			},
			{
				// Re-applying the identical config must be a no-op.
				Config:   testAccIcebergTableOmittedIDsConfig(providerCfg, tableName),
				PlanOnly: true,
			},
		},
	})
}

func testAccIcebergTableInsertFieldConfig(providerCfg, tableName string, withMiddle bool) string {
	middle := ""
	if withMiddle {
		middle = `
      {
        name     = "middle"
        type     = "int"
        required = false
      },`
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_insert" {
  name = ["db_insert"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_insert.name
  name      = "%s"
  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      },%s
      {
        name     = "data"
        type     = "string"
        required = false
      },
      {
        name     = "tags"
        type     = "list"
        required = false
        list_properties = {
          element_type     = "string"
          element_required = true
        }
      }
    ]
  }
}
`, tableName, middle)
}

func TestAccIcebergTableInsertField(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "insert_field_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableInsertFieldConfig(providerCfg, tableName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.id", "3"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.list_properties.element_id", "4"),
				),
			},
			{
				// Inserting a field in the middle must keep every existing
				// column's ID and give only the new column a fresh one.
				Config: testAccIcebergTableInsertFieldConfig(providerCfg, tableName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.name", "middle"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "5"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.name", "data"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.3.name", "tags"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.3.id", "3"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.3.list_properties.element_id", "4"),
				),
			},
			{
				Config:   testAccIcebergTableInsertFieldConfig(providerCfg, tableName, true),
				PlanOnly: true,
			},
		},
	})
}

func testAccIcebergTableDuplicateIDsConfig(providerCfg, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_dup" {
  name = ["db_dup"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_dup.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      },
      {
        id       = 1
        name     = "data"
        type     = "string"
        required = false
      }
    ]
  }
}
`, tableName)
}

func TestAccIcebergTableDuplicateFieldIDs(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "duplicate_ids_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Two fields sharing an explicit id must fail before commit.
				Config:      testAccIcebergTableDuplicateIDsConfig(providerCfg, tableName),
				ExpectError: regexp.MustCompile(`(?s)invalid field id.*reuses id 1`),
			},
		},
	})
}

func testAccIcebergTableEvolutionConfig(providerCfg, tableName, dataType string, dataRequired bool) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_evo" {
  name = ["db_evo"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_evo.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      },
      {
        id       = 2
        name     = "data"
        type     = "%s"
        required = %t
      }
    ]
  }
}
`, tableName, dataType, dataRequired)
}

func testAccIcebergTableExtraFieldConfig(providerCfg, tableName string, extraID int64) string {
	extra := ""
	if extraID != 0 {
		extra = fmt.Sprintf(`,
      {
        id       = %d
        name     = "extra"
        type     = "string"
        required = false
      }`, extraID)
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_reserved" {
  name = ["db_reserved"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_reserved.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      }%s
    ]
  }
}
`, tableName, extra)
}

func TestAccIcebergTableReservedFieldIDs(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "reserved_ids_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableExtraFieldConfig(providerCfg, tableName, 0),
				Check:  resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
			},
			{
				// The spec reserves ids above 2147483447.
				Config:      testAccIcebergTableExtraFieldConfig(providerCfg, tableName, 2147483448),
				ExpectError: regexp.MustCompile(`(?s)invalid field id.*id 2147483448`),
			},
			{
				Config:      testAccIcebergTableExtraFieldConfig(providerCfg, tableName, 2147483647),
				ExpectError: regexp.MustCompile(`(?s)invalid field id.*id 2147483647`),
			},
			{
				// Neither rejected id reached the catalog.
				Config: testAccIcebergTableExtraFieldConfig(providerCfg, tableName, 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
				),
			},
			{
				// The highest non-reserved id is accepted.
				Config: testAccIcebergTableExtraFieldConfig(providerCfg, tableName, 2147483447),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "2147483447"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "1"),
				),
			},
		},
	})
}

func TestAccIcebergTableInvalidEvolution(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "invalid_evolution_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableEvolutionConfig(providerCfg, tableName, "long", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.type", "long"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
				),
			},
			{
				// long -> string is not a valid Iceberg type promotion.
				Config:      testAccIcebergTableEvolutionConfig(providerCfg, tableName, "string", false),
				ExpectError: regexp.MustCompile(`(?s)invalid schema evolution.*not a valid type promotion`),
			},
			{
				// optional -> required is not an allowed schema evolution.
				Config:      testAccIcebergTableEvolutionConfig(providerCfg, tableName, "long", true),
				ExpectError: regexp.MustCompile(`(?s)invalid schema evolution.*optional to required`),
			},
			{
				// Neither rejected change reached the catalog, so the table is
				// still on its original schema version.
				Config: testAccIcebergTableEvolutionConfig(providerCfg, tableName, "long", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.type", "long"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.required", "false"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
				),
			},
		},
	})
}

func TestAccIcebergTableValidEvolution(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "valid_evolution_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableEvolutionConfig(providerCfg, tableName, "int", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.type", "int"),
				),
			},
			{
				Config: testAccIcebergTableEvolutionConfig(providerCfg, tableName, "long", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.type", "long"),
				),
			},
		},
	})
}

func TestAccIcebergTablePropertiesUpdate(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "prop_update_test_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTablePropertiesConfig(providerCfg, tableName, `owner = "initial"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "user_properties.owner", "initial"),
				),
			},
			{
				Config: testAccIcebergTablePropertiesConfig(providerCfg, tableName, `owner = "updated", new_prop = "added"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "user_properties.owner", "updated"),
					resource.TestCheckResourceAttr("iceberg_table.test", "user_properties.new_prop", "added"),
				),
			},
			{
				Config: testAccIcebergTablePropertiesConfig(providerCfg, tableName, `owner = "updated"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "user_properties.owner", "updated"),
					resource.TestCheckNoResourceAttr("iceberg_table.test", "user_properties.new_prop"),
				),
			},
		},
	})
}

func testAccIcebergTablePropertiesConfig(providerCfg string, tableName string, props string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db2" {
  name = ["db2"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db2.name
  name      = "%s"
  user_properties = {
    %s
  }
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
`, tableName, props)
}

func TestAccIcebergTableFull(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableFullConfig(providerCfg, "full_test_table"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.full", "name", "full_test_table"),
					resource.TestCheckResourceAttr("iceberg_table.full", "namespace.0", "full_db"),
					resource.TestCheckResourceAttr("iceberg_table.full", "user_properties.owner", "terraform"),
					resource.TestCheckResourceAttr("iceberg_table.full", "user_properties.env", "test"),

					// Check schema fields
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.0.name", "id"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.0.id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.0.type", "int"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.0.required", "true"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.0.doc", "The unique identifier"),

					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.1.name", "data"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.1.id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.1.type", "string"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.1.required", "false"),

					// Check decimal and fixed
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.2.name", "price"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.2.id", "3"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.2.type", "decimal(10,2)"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.3.name", "hash"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.3.id", "4"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.3.type", "fixed[16]"),

					// Check nested struct
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.name", "location"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.id", "5"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.type", "struct"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.struct_properties.fields.0.name", "lat"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.struct_properties.fields.0.id", "8"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.struct_properties.fields.0.type", "double"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.struct_properties.fields.1.name", "long"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.struct_properties.fields.1.id", "9"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.4.struct_properties.fields.1.type", "double"),

					// Check list
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.5.name", "tags"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.5.id", "6"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.5.type", "list"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.5.list_properties.element_id", "10"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.5.list_properties.element_type", "string"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.5.list_properties.element_required", "true"),

					// Check map
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.name", "metadata"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.id", "7"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.type", "map"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.map_properties.key_id", "11"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.map_properties.key_type", "string"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.map_properties.value_id", "12"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.map_properties.value_type", "int"),
					resource.TestCheckResourceAttr("iceberg_table.full", "schema.fields.6.map_properties.value_required", "false"),
				),
			},
		},
	})
}

func testAccIcebergTableFullConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "full_db" {
  name = ["full_db"]
}

resource "iceberg_table" "full" {
  namespace = iceberg_namespace.full_db.name
  name      = "%s"

  user_properties = {
    owner = "terraform"
    env   = "test"
  }

  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "int"
        required = true
        doc      = "The unique identifier"
      },
      {
        id       = 2
        name     = "data"
        type     = "string"
        required = false
      },
      {
        id       = 3
        name     = "price"
        type     = "decimal(10,2)"
        required = true
      },
      {
        id       = 4
        name     = "hash"
        type     = "fixed[16]"
        required = true
      },
      {
        id       = 5
        name     = "location"
        type     = "struct"
        required = true
        struct_properties = {
          fields = [
            {
              id       = 8
              name     = "lat"
              type     = "double"
              required = true
            },
            {
              id       = 9
              name     = "long"
              type     = "double"
              required = true
            }
          ]
        }
      },
      {
        id       = 6
        name     = "tags"
        type     = "list"
        required = false
        list_properties = {
          element_id       = 10
          element_type     = "string"
          element_required = true
        }
      },
      {
        id       = 7
        name     = "metadata"
        type     = "map"
        required = false
        map_properties = {
          key_id         = 11
          key_type       = "string"
          value_id       = 12
          value_type     = "int"
          value_required = false
        }
      }
    ]
  }
}
`, tableName)
}

func TestAccIcebergTablePartitionSpecAndSortOrder(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "partition_sort_test_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTablePartitionSortConfig(providerCfg, tableName, "bucket[16]", "asc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", tableName),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.field_id", "1000"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.name", "id_bucket"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.transform", "bucket[16]"),

					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.transform", "identity"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.direction", "asc"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.null_order", "nulls-first"),
				),
			},
			{
				Config: testAccIcebergTablePartitionSortConfig(providerCfg, tableName, "bucket[32]", "desc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", tableName),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.transform", "bucket[32]"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.direction", "desc"),
				),
			},
			{
				Config:      testAccIcebergTablePartitionSortConfig(providerCfg, tableName, "bucket[32]", "invalid"),
				ExpectError: regexp.MustCompile(`(?s)Attribute sort_order\.fields\[0\]\.direction value must be one of:.*"asc".*"desc".*got: "invalid"`),
			},
			{
				Config:      testAccIcebergTableInvalidNullOrderConfig(providerCfg, tableName),
				ExpectError: regexp.MustCompile(`(?s)Attribute sort_order\.fields\[0\]\.null_order value must be one of:.*"nulls-first".*"nulls-last".*got: "invalid"`),
			},
			{
				Config: testAccIcebergTableNoPartitionSortConfig(providerCfg, tableName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", tableName),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "0"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "0"),
				),
			},
		},
	})
}

func testAccIcebergTableNoPartitionSortConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db3" {
  name = ["db3"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db3.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableInvalidNullOrderConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db3" {
  name = ["db3"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db3.name
  name      = "%s"
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
  sort_order = {
    fields = [
      {
        source_id  = 1
        transform  = "identity"
        direction  = "asc"
        null_order = "invalid"
      }
    ]
  }
}
`, tableName)
}

func testAccIcebergTablePartitionSortConfig(providerCfg string, tableName string, partitionTransform string, sortDirection string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db3" {
  name = ["db3"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db3.name
  name      = "%s"
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
  partition_spec = {
    fields = [
      {
        source_ids = [1]
        field_id   = 1000
        name       = "id_bucket"
        transform  = "%s"
      }
    ]
  }
  sort_order = {
    fields = [
      {
        source_id  = 1
        transform  = "identity"
        direction  = "%s"
        null_order = "nulls-first"
      }
    ]
  }
}
`, tableName, partitionTransform, sortDirection)
}

func TestAccIcebergTableAddPartitionSpec(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableResourceConfig(providerCfg, "partition_add"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "partition_add"),
				),
			},
			{
				Config: testAccIcebergTablePartitionConfig(providerCfg, "partition_add"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.name", "id_partition"),
				),
			},
		},
	})
}

func TestAccIcebergTableRename(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRenameConfig(providerCfg, "rename_test", "id"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_rename.rename_test"),
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "rename_test"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_rename", "rename_test"}, &tableUUID),
				),
			},
			{
				// The rename is an in-place update, not a re-create.
				Config: testAccIcebergTableRenameConfig(providerCfg, "rename_test_new", "id_new"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("iceberg_table.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_rename.rename_test_new"),
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "rename_test_new"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id_new"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_rename", "rename_test_new"}, &tableUUID),
					testAccCheckIcebergTableMissing(catalogURI, table.Identifier{"db_rename", "rename_test"}),
				),
			},
		},
	})
}

func TestAccIcebergTableNamespaceChangeReplaces(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableResourceConfig(providerCfg, "move_test"),
				Check:  resource.TestCheckResourceAttr("iceberg_table.test", "id", "db1.move_test"),
			},
			{
				Config: testAccIcebergTableRenameConfig(providerCfg, "move_test", "id"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("iceberg_table.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_rename.move_test"),
					resource.TestCheckResourceAttr("iceberg_table.test", "namespace.0", "db_rename"),
				),
			},
		},
	})
}

func TestAccIcebergTableRenameToExistingTable(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRenameConflictConfig(providerCfg, "conflict_a"),
			},
			{
				Config:      testAccIcebergTableRenameConflictConfig(providerCfg, "conflict_b"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The failed rename changes nothing.
				Config:   testAccIcebergTableRenameConflictConfig(providerCfg, "conflict_a"),
				PlanOnly: true,
			},
		},
	})
}

func TestAccIcebergTableRenameThenFailedUpdate(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableCommitConfig(providerCfg, "commit_test", "a"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_commit", "commit_test"}, &tableUUID),
			},
			{
				// The rename succeeds, then the property commit fails.
				PreConfig:   func() { proxy.failCommitsTo("commit_test_new") },
				Config:      testAccIcebergTableCommitConfig(providerCfg, "commit_test_new", "b"),
				ExpectError: regexp.MustCompile(`failed to commit table updates`),
			},
			{
				// The state already has the new name, so the retry only commits.
				PreConfig: func() { proxy.failCommitsTo("") },
				Config:    testAccIcebergTableCommitConfig(providerCfg, "commit_test_new", "b"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_commit.commit_test_new"),
					resource.TestCheckResourceAttr("iceberg_table.test", "user_properties.owner", "b"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_commit", "commit_test_new"}, &tableUUID),
				),
			},
		},
	})
}

func TestAccIcebergTableRenameWithInvalidSchemaChange(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRenameRequiredConfig(providerCfg, "partial_test", false),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_partial", "partial_test"}, &tableUUID),
			},
			{
				// Making an optional column required is rejected before the rename.
				Config:      testAccIcebergTableRenameRequiredConfig(providerCfg, "partial_test_new", true),
				ExpectError: regexp.MustCompile(`invalid schema evolution`),
			},
			{
				PreConfig: func() {
					require.NoError(t, testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_partial", "partial_test"}, &tableUUID)(nil))
					require.NoError(t, testAccCheckIcebergTableMissing(catalogURI, table.Identifier{"db_partial", "partial_test_new"})(nil))
				},
				Config:   testAccIcebergTableRenameRequiredConfig(providerCfg, "partial_test", false),
				PlanOnly: true,
			},
		},
	})
}

func TestAccIcebergTableRenameReloadFails(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRenameReloadConfig(providerCfg, "reload_test"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_reload", "reload_test"}, &tableUUID),
			},
			{
				// The check that the new name is free and RenameTable's reload both
				// fail; the provider finds the table under the new name and
				// finishes the update.
				PreConfig: func() { proxy.fail("reload_test_new", 2) },
				Config:    testAccIcebergTableRenameReloadConfig(providerCfg, "reload_test_new"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_reload.reload_test_new"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_reload", "reload_test_new"}, &tableUUID),
				),
			},
			{
				// The renamed table cannot be loaded, so the outcome is unknown.
				PreConfig:   func() { proxy.fail("reload_test_new2", -1) },
				Config:      testAccIcebergTableRenameReloadConfig(providerCfg, "reload_test_new2"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// A refresh that cannot load the new name keeps the resource.
				RefreshState: true,
				ExpectError:  regexp.MustCompile(`failed to load table`),
			},
			{
				// Another table now has the old name; the refresh skips it and
				// finds the original table under the new name.
				PreConfig: func() {
					proxy.fail("", 0)
					testAccCreateIcebergTable(t, catalogURI, table.Identifier{"db_reload", "reload_test_new"})
				},
				Config: testAccIcebergTableRenameReloadConfig(providerCfg, "reload_test_new2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_reload.reload_test_new2"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_reload", "reload_test_new2"}, &tableUUID),
					testAccCheckIcebergTableOtherUUID(catalogURI, table.Identifier{"db_reload", "reload_test_new"}, &tableUUID),
				),
			},
			{
				PreConfig: func() { testAccDropIcebergTable(t, catalogURI, table.Identifier{"db_reload", "reload_test_new"}) },
				Config:    testAccIcebergTableRenameReloadConfig(providerCfg, "reload_test_new2"),
				PlanOnly:  true,
			},
		},
	})
}

func TestAccIcebergTableRenameDestroyWithoutRefresh(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	namespace := table.Identifier{"db_norefresh"}
	oldIdent := table.Identifier{"db_norefresh", "norefresh_test"}
	newIdent := table.Identifier{"db_norefresh", "norefresh_test_new"}
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		AdditionalCLIOptions:     &resource.AdditionalCLIOptions{Plan: resource.PlanOptions{NoRefresh: true}},
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccCreateIcebergNamespace(t, catalogURI, namespace)
					t.Cleanup(func() {
						testAccDropIcebergTable(t, catalogURI, oldIdent)
						testAccDropIcebergTable(t, catalogURI, newIdent)
						cat, err := rest.NewCatalog(context.Background(), "rest", catalogURI)
						require.NoError(t, err)
						require.NoError(t, cat.DropNamespace(context.Background(), namespace))
					})
				},
				Config: testAccIcebergTableNoRefreshConfig(providerCfg, "norefresh_test"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_norefresh", "norefresh_test"}, &tableUUID),
			},
			{
				PreConfig:   func() { proxy.fail("norefresh_test_new", -1) },
				Config:      testAccIcebergTableNoRefreshConfig(providerCfg, "norefresh_test_new"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The state still has the old name, which another table now
				// uses. A Check would refresh the state first, so the result is
				// checked in the next step.
				PreConfig: func() {
					proxy.fail("", 0)
					testAccCreateIcebergTable(t, catalogURI, oldIdent)
				},
				Config:  testAccIcebergTableNoRefreshConfig(providerCfg, "norefresh_test_new"),
				Destroy: true,
			},
			{
				// Destroy dropped only the original table.
				PreConfig: func() {
					require.NoError(t, testAccCheckIcebergTableMissing(catalogURI, newIdent)(nil))
					require.NoError(t, testAccCheckIcebergTableOtherUUID(catalogURI, oldIdent, &tableUUID)(nil))
				},
				Config:   providerCfg,
				PlanOnly: true,
			},
		},
	})
}

func TestTableIDPlanModifier(t *testing.T) {
	ctx := context.Background()
	schemaResp := &fwresource.SchemaResponse{}
	NewTableResource().Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	listType := tftypes.List{ElementType: tftypes.String}
	object := func(namespace tftypes.Value, name string) tftypes.Value {
		values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
		for attr, attrType := range objectType.AttributeTypes {
			values[attr] = tftypes.NewValue(attrType, nil)
		}
		values["namespace"] = namespace
		values["name"] = tftypes.NewValue(tftypes.String, name)

		return tftypes.NewValue(objectType, values)
	}
	db := tftypes.NewValue(listType, []tftypes.Value{tftypes.NewValue(tftypes.String, "db")})
	unknownDB := tftypes.NewValue(listType, []tftypes.Value{tftypes.NewValue(tftypes.String, tftypes.UnknownValue)})

	for _, tc := range []struct {
		name      string
		namespace tftypes.Value
		tableName string
		want      types.String
	}{
		{"unchanged", db, "events", types.StringValue("db.events")},
		{"renamed", db, "events_new", types.StringValue("db.events_new")},
		{"unknown namespace element", unknownDB, "events_new", types.StringUnknown()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				Path:        path.Root("id"),
				Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: object(tc.namespace, tc.tableName)},
				State:       tfsdk.State{Schema: schemaResp.Schema, Raw: object(db, "events")},
				ConfigValue: types.StringNull(),
				PlanValue:   types.StringUnknown(),
				StateValue:  types.StringValue("db.events"),
			}
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
			tableIDPlanModifier{}.PlanModifyString(ctx, req, resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			require.Equal(t, tc.want, resp.PlanValue)
		})
	}
}

func TestAccIcebergTableRenameAfterUnknownRename(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		AdditionalCLIOptions:     &resource.AdditionalCLIOptions{Plan: resource.PlanOptions{NoRefresh: true}},
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableChainConfig(providerCfg, "chain_a"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_chain", "chain_a"}, &tableUUID),
			},
			{
				// chain_a -> chain_b happens, but its outcome is unknown.
				PreConfig:   func() { proxy.fail("chain_b", -1) },
				Config:      testAccIcebergTableChainConfig(providerCfg, "chain_b"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// Without a refresh, the table is found under chain_b, and the
				// rename to chain_c fails before reaching the catalog.
				PreConfig: func() {
					proxy.fail("", 0)
					proxy.failRename(true)
				},
				Config:      testAccIcebergTableChainConfig(providerCfg, "chain_c"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The table is still tracked under chain_b, so the retry works.
				PreConfig: func() { proxy.failRename(false) },
				Config:    testAccIcebergTableChainConfig(providerCfg, "chain_c"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_chain.chain_c"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_chain", "chain_c"}, &tableUUID),
				),
			},
		},
	})
}

func TestAccIcebergTableRenameOldNameUnreadable(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableUnreadableConfig(providerCfg, "unreadable_test"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_unreadable", "unreadable_test"}, &tableUUID),
			},
			{
				PreConfig:   func() { proxy.fail("unreadable_test_new", -1) },
				Config:      testAccIcebergTableUnreadableConfig(providerCfg, "unreadable_test_new"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The old name cannot be read, but the table is found under the new name.
				PreConfig:    func() { proxy.fail("unreadable_test", -1) },
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_unreadable.unreadable_test_new"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_unreadable", "unreadable_test_new"}, &tableUUID),
				),
			},
			{
				PreConfig: func() { proxy.fail("", 0) },
				Config:    testAccIcebergTableUnreadableConfig(providerCfg, "unreadable_test_new"),
				PlanOnly:  true,
			},
		},
	})
}

func TestAccIcebergTableImportedIDKeptOnUpdate(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	namespace := table.Identifier{"db_import_id"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccCreateIcebergNamespace(t, catalogURI, namespace)
					testAccCreateIcebergTable(t, catalogURI, table.Identifier{"db_import_id", "events"})
					t.Cleanup(func() {
						testAccDropIcebergTable(t, catalogURI, table.Identifier{"db_import_id", "events"})
						cat, err := rest.NewCatalog(context.Background(), "rest", catalogURI)
						require.NoError(t, err)
						require.NoError(t, cat.DropNamespace(context.Background(), namespace))
					})
				},
				Config:             testAccIcebergTableImportIDConfig(providerCfg, "a"),
				ResourceName:       "iceberg_table.test",
				ImportState:        true,
				ImportStateId:      "db_import_id..events",
				ImportStatePersist: true,
			},
			{
				// A property update keeps the imported ID.
				Config: testAccIcebergTableImportIDConfig(providerCfg, "b"),
				Check:  resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_import_id..events"),
			},
		},
	})
}

func TestHasTableUUID(t *testing.T) {
	withUUID := testTable(t, testTableUUID)
	require.True(t, hasTableUUID(withUUID, testTableUUID))
	require.False(t, hasTableUUID(withUUID, "a0b1c2d3-03fe-4693-9a96-a0705ddf69c1"))

	// A table without a UUID never matches, not even another table without one.
	withoutUUID := testTable(t, "")
	require.False(t, hasTableUUID(withoutUUID, withoutUUID.Metadata().TableUUID().String()))
}

func TestAccIcebergTableRenameBlockedWhilePending(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableMultiConfig(providerCfg, "multi_a"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_multi", "multi_a"}, &tableUUID),
			},
			{
				// multi_a -> multi_b fails with an unknown outcome.
				PreConfig:   func() { proxy.failRename(true) },
				Config:      testAccIcebergTableMultiConfig(providerCfg, "multi_b"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// While that rename is pending, the provider does not rename the
				// table to another name.
				PreConfig:   func() { proxy.failRename(false) },
				Config:      testAccIcebergTableMultiConfig(providerCfg, "multi_c"),
				ExpectError: regexp.MustCompile(`unknown outcome`),
			},
			{
				// The first rename completes late; the refresh finds the table.
				PreConfig: func() {
					testAccRenameIcebergTable(t, catalogURI, table.Identifier{"db_multi", "multi_a"}, table.Identifier{"db_multi", "multi_b"})
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "multi_b"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_multi", "multi_b"}, &tableUUID),
				),
			},
			{
				Config: testAccIcebergTableMultiConfig(providerCfg, "multi_c"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_multi.multi_c"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_multi", "multi_c"}, &tableUUID),
				),
			},
		},
	})
}

func TestAccIcebergTableRenameToView(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	viewIdent := table.Identifier{"db_view", "taken_view"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableViewConfig(providerCfg, "view_test"),
			},
			{
				// A view with the new name stops the rename before it is sent.
				PreConfig:   func() { testAccCreateIcebergView(t, catalogURI, viewIdent) },
				Config:      testAccIcebergTableViewConfig(providerCfg, "taken_view"),
				ExpectError: regexp.MustCompile(`view already exists: db_view\.taken_view`),
			},
			{
				PreConfig: func() { testAccDropIcebergView(t, catalogURI, viewIdent) },
				Config:    testAccIcebergTableViewConfig(providerCfg, "view_test"),
				PlanOnly:  true,
			},
		},
	})
}

func TestAccIcebergTableRenameWithoutUUIDRefused(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableNoUUIDConfig(providerCfg, "nouuid_a"),
			},
			{
				// The table now looks like a format-version 1 table without a UUID.
				PreConfig:   func() { proxy.serveWithoutUUID(true) },
				Config:      testAccIcebergTableNoUUIDConfig(providerCfg, "nouuid_b"),
				ExpectError: regexp.MustCompile(`no UUID, so it cannot be renamed in place`),
			},
			{
				// No rename was sent, and the table is still managed under its name.
				PreConfig: func() { require.Zero(t, proxy.renameRequests()) },
				Config:    testAccIcebergTableNoUUIDConfig(providerCfg, "nouuid_a"),
				PlanOnly:  true,
			},
		},
	})
}

func TestAccIcebergTableRenameRejected(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRejectConfig(providerCfg, "reject_a"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_reject", "reject_a"}, &tableUUID),
			},
			{
				PreConfig:   func() { proxy.failRenameWith(http.StatusForbidden) },
				Config:      testAccIcebergTableRejectConfig(providerCfg, "reject_b"),
				ExpectError: regexp.MustCompile(`ForbiddenException`),
			},
			{
				// The 403 was not recorded, so renaming to another name is allowed.
				PreConfig:   func() { proxy.failRenameWith(http.StatusConflict) },
				Config:      testAccIcebergTableRejectConfig(providerCfg, "reject_c"),
				ExpectError: regexp.MustCompile(`AlreadyExistsException`),
			},
			{
				PreConfig: func() { proxy.failRenameWith(0) },
				Config:    testAccIcebergTableRejectConfig(providerCfg, "reject_d"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_reject.reject_d"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_reject", "reject_d"}, &tableUUID),
				),
			},
		},
	})
}

func TestAccIcebergTableDestroyRefusedWhilePending(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableDestroyConfig(providerCfg, "destroy_a"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_destroy", "destroy_a"}, &tableUUID),
			},
			{
				PreConfig:   func() { proxy.failRename(true) },
				Config:      testAccIcebergTableDestroyConfig(providerCfg, "destroy_b"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The table is still under its old name, so it is not dropped by it.
				PreConfig:   func() { proxy.failRename(false) },
				Config:      testAccIcebergTableDestroyConfig(providerCfg, "destroy_b"),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`not\s+dropped\s+by\s+its\s+old\s+name`),
			},
			{
				// Retrying the rename resolves it.
				Config: testAccIcebergTableDestroyConfig(providerCfg, "destroy_b"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_destroy", "destroy_b"}, &tableUUID),
			},
		},
	})
}

func TestAccIcebergTableRenameRetrySameTarget(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRetryConfig(providerCfg, "retry_a"),
				Check:  testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_retry", "retry_a"}, &tableUUID),
			},
			{
				PreConfig:   func() { proxy.failRename(true) },
				Config:      testAccIcebergTableRetryConfig(providerCfg, "retry_b"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The same rename can be retried while its outcome is unknown.
				PreConfig: func() { proxy.failRename(false) },
				Config:    testAccIcebergTableRetryConfig(providerCfg, "retry_b"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_retry.retry_b"),
					testAccCheckIcebergTableUUID(catalogURI, table.Identifier{"db_retry", "retry_b"}, &tableUUID),
				),
			},
		},
	})
}

func TestAccIcebergTableRenameCompletesLate(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	proxy := newFailingTableProxy(t, catalogURI)
	providerCfg := fmt.Sprintf(providerConfig, proxy.URL)
	namespace := table.Identifier{"db_late"}
	oldIdent := table.Identifier{"db_late", "late_a"}
	newIdent := table.Identifier{"db_late", "late_b"}
	var tableUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					testAccCreateIcebergNamespace(t, catalogURI, namespace)
					testAccCreateIcebergTable(t, catalogURI, oldIdent)
					require.NoError(t, testAccCheckIcebergTableUUID(catalogURI, oldIdent, &tableUUID)(nil))
					t.Cleanup(func() {
						testAccDropIcebergTable(t, catalogURI, oldIdent)
						testAccDropIcebergTable(t, catalogURI, newIdent)
						cat, err := rest.NewCatalog(context.Background(), "rest", catalogURI)
						require.NoError(t, err)
						require.NoError(t, cat.DropNamespace(context.Background(), namespace))
					})
				},
				Config:             testAccIcebergTableLateConfig(providerCfg, "late_a"),
				ResourceName:       "iceberg_table.test",
				ImportState:        true,
				ImportStateId:      "db_late..late_a",
				ImportStatePersist: true,
			},
			{
				// The rename request fails, so its outcome is unknown.
				PreConfig:   func() { proxy.failRename(true) },
				Config:      testAccIcebergTableLateConfig(providerCfg, "late_b"),
				ExpectError: regexp.MustCompile(`failed to rename table`),
			},
			{
				// The table is still under its old name; the imported ID is kept.
				PreConfig:          func() { proxy.failRename(false) },
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "late_a"),
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_late..late_a"),
				),
			},
			{
				// The rename completes later; the next refresh still follows it.
				PreConfig:    func() { testAccRenameIcebergTable(t, catalogURI, oldIdent, newIdent) },
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "id", "db_late.late_b"),
					testAccCheckIcebergTableUUID(catalogURI, newIdent, &tableUUID),
				),
			},
		},
	})
}

// fakeCatalog keeps tables in a map; methods it does not override are not implemented.
type fakeCatalog struct {
	catalog.Catalog
	tables map[string]*table.Table
	views  map[string]bool
	// renameErr is returned by RenameTable, after the rename when renameMoves is set.
	renameErr   error
	renameMoves bool
	// replaceOnRename, if set, takes the new name right after a successful rename.
	replaceOnRename *table.Table
	renames         int
	// beforeDrop runs at the start of DropTable.
	beforeDrop func(ident string)
}

func (c *fakeCatalog) LoadTable(_ context.Context, ident table.Identifier) (*table.Table, error) {
	if tbl, ok := c.tables[strings.Join(ident, ".")]; ok {
		return tbl, nil
	}

	return nil, catalog.ErrNoSuchTable
}

func (c *fakeCatalog) RenameTable(_ context.Context, from, to table.Identifier) (*table.Table, error) {
	c.renames++
	if c.renameErr == nil || c.renameMoves {
		c.move(strings.Join(from, "."), strings.Join(to, "."))
	}
	if c.renameErr != nil {
		return nil, c.renameErr
	}
	if c.replaceOnRename != nil {
		c.tables[strings.Join(to, ".")] = c.replaceOnRename
	}

	return c.tables[strings.Join(to, ".")], nil
}

func (c *fakeCatalog) CheckViewExists(_ context.Context, ident table.Identifier) (bool, error) {
	return c.views[strings.Join(ident, ".")], nil
}

func (c *fakeCatalog) DropTable(_ context.Context, ident table.Identifier) error {
	key := strings.Join(ident, ".")
	if c.beforeDrop != nil {
		c.beforeDrop(key)
	}
	if _, ok := c.tables[key]; !ok {
		return catalog.ErrNoSuchTable
	}
	delete(c.tables, key)

	return nil
}

func (c *fakeCatalog) move(from, to string) {
	if tbl, ok := c.tables[from]; ok {
		delete(c.tables, from)
		c.tables[to] = tbl
	}
}

// testTable returns a format-version 1 table, without a UUID if tableUUID is empty.
func testTable(t *testing.T, tableUUID string) *table.Table {
	extra := ""
	if tableUUID != "" {
		extra = `"table-uuid": "` + tableUUID + `",`
	}
	meta, err := table.ParseMetadataString(`{"format-version": 1, "location": "memory://t", "last-updated-ms": 1,
  "last-column-id": 1, "partition-spec": [], ` + extra + `
  "schema": {"type": "struct", "schema-id": 0, "fields": [{"id": 1, "name": "id", "type": "long", "required": true}]}}`)
	require.NoError(t, err)

	return table.New(table.Identifier{"db", "t"}, meta, "", nil, nil)
}

const testTableUUID = "9c12d441-03fe-4693-9a96-a0705ddf69c1"

func TestRenameTableOutcome(t *testing.T) {
	timeout := errors.New("timeout")
	for _, tc := range []struct {
		name            string
		tableUUID       string
		to              string
		renameErr       error
		renameMoves     bool
		replaceOnRename *table.Table
		wantRenamed     bool
		wantUnsent      bool
		wantRecord      *pendingRename
	}{
		{name: "renamed", tableUUID: testTableUUID, to: "c", wantRenamed: true},
		{name: "table without a UUID is not renamed", to: "c", wantUnsent: true},
		{name: "invalid name is never sent", tableUUID: testTableUUID, to: "bad/name", wantUnsent: true},
		{name: "name taken is never sent", tableUUID: testTableUUID, to: "taken", renameErr: timeout, renameMoves: true, wantUnsent: true},
		{name: "name of a view is never sent", tableUUID: testTableUUID, to: "view1", renameErr: timeout, renameMoves: true, wantUnsent: true},
		{name: "endpoint not supported", tableUUID: testTableUUID, to: "c", renameErr: rest.ErrEndpointNotSupported},
		{name: "already exists", tableUUID: testTableUUID, to: "c", renameErr: catalog.ErrTableAlreadyExists},
		{name: "rejected with 403", tableUUID: testTableUUID, to: "c", renameErr: rest.ErrForbidden},
		{name: "rejected with an unmapped 409", tableUUID: testTableUUID, to: "c", renameErr: rest.ErrRESTError},
		{
			name: "server error", tableUUID: testTableUUID, to: "c", renameErr: rest.ErrServerError,
			wantRecord: &pendingRename{Names: []string{"b", "c"}, TableUUID: testTableUUID},
		},
		{
			name: "403 while reloading the renamed table", tableUUID: testTableUUID, to: "c",
			renameErr: rest.ErrForbidden, renameMoves: true, wantRenamed: true,
		},
		{
			name: "unknown outcome", tableUUID: testTableUUID, to: "c", renameErr: timeout,
			wantRecord: &pendingRename{Names: []string{"b", "c"}, TableUUID: testTableUUID},
		},
		{
			name: "renamed but the reload failed", tableUUID: testTableUUID, to: "c",
			renameErr: timeout, renameMoves: true, wantRenamed: true,
		},
		{
			name: "another table under the new name", tableUUID: testTableUUID, to: "c",
			replaceOnRename: testTable(t, "a0b1c2d3-03fe-4693-9a96-a0705ddf69c1"),
			wantRecord:      &pendingRename{Names: []string{"b", "c"}, TableUUID: testTableUUID},
		},
		{
			name: "retrying a pending rename", tableUUID: testTableUUID, to: "b", renameErr: timeout,
			wantRecord: &pendingRename{Names: []string{"b"}, TableUUID: testTableUUID},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl := testTable(t, tc.tableUUID)
			cat := &fakeCatalog{
				tables:    map[string]*table.Table{"db.a": tbl, "db.taken": testTable(t, "")},
				views:     map[string]bool{"db.view1": true},
				renameErr: tc.renameErr, renameMoves: tc.renameMoves, replaceOnRename: tc.replaceOnRename,
			}
			r := &icebergTableResource{catalog: cat}

			// "b" is the destination of an earlier rename with an unknown outcome.
			renamed, record, err := r.renameTable(context.Background(), tbl, table.Identifier{"db", "a"}, table.Identifier{"db", tc.to}, []string{"b"})
			require.Equal(t, tc.wantRenamed, err == nil, "%v", err)
			require.Equal(t, tc.wantRenamed, renamed != nil)
			require.Equal(t, tc.wantRecord, record)
			if tc.wantUnsent {
				require.Zero(t, cat.renames, "the rename request was sent")
			}
		})
	}
}

func TestDropPendingTable(t *testing.T) {
	pending := &pendingRename{Names: []string{"b"}, TableUUID: testTableUUID}

	// While the table is still under its old name, it is not dropped by that name.
	cat := &fakeCatalog{tables: map[string]*table.Table{"db.a": testTable(t, testTableUUID)}}
	r := &icebergTableResource{catalog: cat}
	require.ErrorContains(t, r.dropPendingTable(context.Background(), []string{"db"}, "a", pending), "unknown outcome")
	require.Contains(t, cat.tables, "db.a")

	// Under the new name it is dropped.
	cat = &fakeCatalog{tables: map[string]*table.Table{"db.b": testTable(t, testTableUUID)}}
	r = &icebergTableResource{catalog: cat}
	require.NoError(t, r.dropPendingTable(context.Background(), []string{"db"}, "a", pending))
	require.Empty(t, cat.tables)
}

// failingTableProxy forwards requests to the catalog, failing GETs of one table
// to simulate RenameTable failing to load the table it has just renamed.
type failingTableProxy struct {
	*httptest.Server
	mu           sync.Mutex
	tableName    string
	remaining    int // -1 fails every request
	renameStatus int // status returned for rename requests; 0 forwards them
	renames      int
	failCommits  string // fail commits to this table
	withoutUUID  bool
}

func newFailingTableProxy(t *testing.T, catalogURI string) *failingTableProxy {
	target, err := url.Parse(catalogURI)
	require.NoError(t, err)
	p := &failingTableProxy{}
	forward := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Header.Del("Accept-Encoding") // keep responses readable for rewriteWithoutUUID
		},
		ModifyResponse: p.rewriteWithoutUUID,
	}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := 0
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/tables/rename"):
			status = p.renameStatusCode()
		case r.Method == http.MethodPost && p.commitFails(r.URL.Path):
			status = http.StatusServiceUnavailable
		case r.Method == http.MethodGet && p.shouldFail(r.URL.Path):
			status = http.StatusServiceUnavailable
		}
		if status != 0 {
			errorType := map[int]string{
				http.StatusForbidden:          "ForbiddenException",
				http.StatusConflict:           "AlreadyExistsException",
				http.StatusServiceUnavailable: "ServiceUnavailableException",
			}[status]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, `{"error":{"message":"injected failure","type":%q,"code":%d}}`, errorType, status)

			return
		}
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(p.Close)

	return p
}

func (p *failingTableProxy) fail(tableName string, times int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tableName, p.remaining = tableName, times
}

func (p *failingTableProxy) failRename(fail bool) {
	p.failRenameWith(map[bool]int{true: http.StatusServiceUnavailable}[fail])
}

// failRenameWith answers rename requests with status; 0 forwards them.
func (p *failingTableProxy) failRenameWith(status int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.renameStatus = status
}

// renameStatusCode counts a rename request and returns the status to answer with.
func (p *failingTableProxy) renameStatusCode() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.renames++

	return p.renameStatus
}

// failCommitsTo fails commits to the named table; "" stops failing them.
func (p *failingTableProxy) failCommitsTo(tableName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failCommits = tableName
}

func (p *failingTableProxy) commitFails(requestPath string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.failCommits != "" && strings.HasSuffix(requestPath, "/tables/"+p.failCommits)
}

func (p *failingTableProxy) renameRequests() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.renames
}

// serveWithoutUUID makes loaded tables look like format-version 1 tables
// without a UUID.
func (p *failingTableProxy) serveWithoutUUID(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.withoutUUID = enabled
}

func (p *failingTableProxy) rewriteWithoutUUID(resp *http.Response) error {
	p.mu.Lock()
	enabled := p.withoutUUID
	p.mu.Unlock()
	if !enabled || resp.Request.Method != http.MethodGet || !strings.Contains(resp.Request.URL.Path, "/tables/") || resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var loaded map[string]any
	if err := json.Unmarshal(body, &loaded); err != nil {
		return err
	}
	meta := loaded["metadata"].(map[string]any)
	meta["format-version"] = 1
	delete(meta, "table-uuid")
	delete(meta, "last-sequence-number")
	for _, schema := range meta["schemas"].([]any) {
		if schema.(map[string]any)["schema-id"] == meta["current-schema-id"] {
			meta["schema"] = schema
		}
	}
	for _, spec := range meta["partition-specs"].([]any) {
		if spec.(map[string]any)["spec-id"] == meta["default-spec-id"] {
			meta["partition-spec"] = spec.(map[string]any)["fields"]
		}
	}
	if body, err = json.Marshal(loaded); err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))

	return nil
}

func (p *failingTableProxy) shouldFail(requestPath string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.remaining == 0 || !strings.HasSuffix(requestPath, "/tables/"+p.tableName) {
		return false
	}
	if p.remaining > 0 {
		p.remaining--
	}

	return true
}

// testAccCheckIcebergTableUUID records the table UUID, then checks it is unchanged.
func testAccCheckIcebergTableUUID(catalogURI string, ident table.Identifier, uuid *string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		ctx := context.Background()
		cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
		if err != nil {
			return err
		}
		tbl, err := cat.LoadTable(ctx, ident)
		if err != nil {
			return err
		}
		got := tbl.Metadata().TableUUID().String()
		if *uuid == "" {
			*uuid = got
		} else if got != *uuid {
			return fmt.Errorf("table %v has UUID %s, expected %s: it was re-created", ident, got, *uuid)
		}

		return nil
	}
}

// testAccCheckIcebergTableOtherUUID checks that ident is a different table.
func testAccCheckIcebergTableOtherUUID(catalogURI string, ident table.Identifier, uuid *string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		ctx := context.Background()
		cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
		if err != nil {
			return err
		}
		tbl, err := cat.LoadTable(ctx, ident)
		if err != nil {
			return err
		}
		if tbl.Metadata().TableUUID().String() == *uuid {
			return fmt.Errorf("table %v is the original table", ident)
		}

		return nil
	}
}

func testAccCreateIcebergNamespace(t *testing.T, catalogURI string, namespace table.Identifier) {
	ctx := context.Background()
	cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
	require.NoError(t, err)
	require.NoError(t, cat.CreateNamespace(ctx, namespace, nil))
}

func testAccCreateIcebergTable(t *testing.T, catalogURI string, ident table.Identifier) {
	ctx := context.Background()
	cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
	require.NoError(t, err)
	schema := iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true})
	_, err = cat.CreateTable(ctx, ident, schema)
	require.NoError(t, err)
}

func testAccDropIcebergTable(t *testing.T, catalogURI string, ident table.Identifier) {
	ctx := context.Background()
	cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
	require.NoError(t, err)
	err = cat.DropTable(ctx, ident)
	if !errors.Is(err, catalog.ErrNoSuchTable) {
		require.NoError(t, err)
	}
}

func testAccRenameIcebergTable(t *testing.T, catalogURI string, from, to table.Identifier) {
	ctx := context.Background()
	cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
	require.NoError(t, err)
	_, err = cat.RenameTable(ctx, from, to)
	require.NoError(t, err)
}

func testAccCreateIcebergView(t *testing.T, catalogURI string, ident table.Identifier) {
	ctx := context.Background()
	cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
	require.NoError(t, err)
	schema := iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true})
	version := &view.Version{
		VersionID:        1,
		TimestampMS:      time.Now().UnixMilli(),
		Summary:          view.VersionSummary{},
		Representations:  []view.Representation{view.NewRepresentation("SELECT 1 AS id", "spark")},
		DefaultNamespace: ident[:len(ident)-1],
	}
	_, err = cat.CreateView(ctx, ident, version, schema)
	require.NoError(t, err)
}

func testAccDropIcebergView(t *testing.T, catalogURI string, ident table.Identifier) {
	ctx := context.Background()
	cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
	require.NoError(t, err)
	require.NoError(t, cat.DropView(ctx, ident))
}

func testAccCheckIcebergTableMissing(catalogURI string, ident table.Identifier) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		ctx := context.Background()
		cat, err := rest.NewCatalog(ctx, "rest", catalogURI)
		if err != nil {
			return err
		}
		_, err = cat.LoadTable(ctx, ident)
		if errors.Is(err, catalog.ErrNoSuchTable) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("table %v still exists", ident)
	}
}

func testAccIcebergTablePartitionConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_partition" {
  name = ["db_partition"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_partition.name
  name      = "%s"
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
  partition_spec = {
    fields = [
      {
        source_ids = [1]
        name       = "id_partition"
        transform  = "identity"
      }
    ]
  }
}
`, tableName)
}

func testAccIcebergTableRenameConfig(providerCfg string, tableName string, colName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_rename" {
  name = ["db_rename"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_rename.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "%s"
        type     = "long"
        required = true
      },
      {
        id       = 2
        name     = "data"
        type     = "string"
        required = false
      }
    ]
  }
}
`, tableName, colName)
}

func testAccIcebergTableRenameConflictConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_conflict" {
  name = ["db_conflict"]
}

resource "iceberg_table" "other" {
  namespace = iceberg_namespace.db_conflict.name
  name      = "conflict_b"
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

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_conflict.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableRenameRequiredConfig(providerCfg string, tableName string, dataRequired bool) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_partial" {
  name = ["db_partial"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_partial.name
  name      = "%s"
  schema = {
    fields = [
      {
        id       = 1
        name     = "id"
        type     = "long"
        required = true
      },
      {
        id       = 2
        name     = "data"
        type     = "string"
        required = %t
      }
    ]
  }
}
`, tableName, dataRequired)
}

func testAccIcebergTableRenameReloadConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_reload" {
  name = ["db_reload"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_reload.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableNoRefreshConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_table" "test" {
  namespace = ["db_norefresh"]
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableChainConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_chain" {
  name = ["db_chain"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_chain.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableUnreadableConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_unreadable" {
  name = ["db_unreadable"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_unreadable.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableImportIDConfig(providerCfg string, owner string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_table" "test" {
  namespace = ["db_import_id"]
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
  user_properties = {
    owner = "%s"
  }
}
`, owner)
}

func testAccIcebergTableLateConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_table" "test" {
  namespace = ["db_late"]
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableMultiConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_multi" {
  name = ["db_multi"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_multi.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableRetryConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_retry" {
  name = ["db_retry"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_retry.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableViewConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_view" {
  name = ["db_view"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_view.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableNoUUIDConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_nouuid" {
  name = ["db_nouuid"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_nouuid.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableRejectConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_reject" {
  name = ["db_reject"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_reject.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableDestroyConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_destroy" {
  name = ["db_destroy"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_destroy.name
  name      = "%s"
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
`, tableName)
}

func testAccIcebergTableCommitConfig(providerCfg string, tableName string, owner string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_commit" {
  name = ["db_commit"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_commit.name
  name      = "%s"
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
  user_properties = {
    owner = "%s"
  }
}
`, tableName, owner)
}
