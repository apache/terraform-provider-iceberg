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
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/apache/iceberg-go/table"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableResourceConfig(providerCfg, "rename_test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "rename_test"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id"),
				),
			},
			{
				Config: testAccIcebergTableRenameConfig(providerCfg, "rename_test_new", "id_new"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "name", "rename_test_new"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.name", "id_new"),
				),
			},
		},
	})
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

func TestAccIcebergTableRevertToExistingMetadata(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "revert_test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRevertConfig(providerCfg, tableName, false, "asc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
					resource.TestCheckNoResourceAttr("iceberg_table.test", "partition_spec"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.order_id", "1"),
				),
			},
			{
				Config: testAccIcebergTableRevertConfig(providerCfg, tableName, true, "desc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.spec_id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.order_id", "2"),
				),
			},
			{
				Config: testAccIcebergTableRevertConfig(providerCfg, tableName, false, "asc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.#", "2"),
					resource.TestCheckNoResourceAttr("iceberg_table.test", "partition_spec"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.order_id", "1"),
				),
			},
		},
	})
}

func testAccIcebergTableRevertConfig(providerCfg, tableName string, evolved bool, sortDirection string) string {
	extraField, partitionSpec := "", ""
	if evolved {
		extraField = `,
      {
        name     = "b"
        type     = "string"
        required = false
      }`
		partitionSpec = `
  partition_spec = {
    fields = [
      {
        source_ids = [2]
        name       = "ts_day"
        transform  = "day"
      }
    ]
  }`
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_revert" {
  name = ["db_revert"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_revert.name
  name      = "%s"
  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      },
      {
        name     = "ts"
        type     = "timestamp"
        required = false
      }%s
    ]
  }%s
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
`, tableName, extraField, partitionSpec, sortDirection)
}

// historyMetadata is a table whose current schema, spec, and sort order are
// the last of several, so the earlier ones can be returned to.
const historyMetadata = `{
  "format-version": 2,
  "table-uuid": "9c12d441-03fe-4693-9a96-a0705ddf69c1",
  "location": "s3://bucket/t",
  "last-sequence-number": 0,
  "last-updated-ms": 1602638573590,
  "last-column-id": 3,
  "current-schema-id": 1,
  "schemas": [
    {"type": "struct", "schema-id": 0, "fields": [
      {"id": 1, "name": "id", "required": true, "type": "long"},
      {"id": 2, "name": "ts", "required": false, "type": "timestamp"}
    ]},
    {"type": "struct", "schema-id": 1, "fields": [
      {"id": 1, "name": "id", "required": true, "type": "long"},
      {"id": 2, "name": "ts", "required": false, "type": "timestamp"},
      {"id": 3, "name": "b", "required": false, "type": "string"}
    ]}
  ],
  "default-spec-id": 1,
  "partition-specs": [
    {"spec-id": 0, "fields": []},
    {"spec-id": 1, "fields": [{"source-id": 2, "field-id": 1000, "name": "ts_day", "transform": "day"}]}
  ],
  "last-partition-id": 1000,
  "default-sort-order-id": 2,
  "sort-orders": [
    {"order-id": 0, "fields": []},
    {"order-id": 1, "fields": [{"source-id": 1, "transform": "identity", "direction": "asc", "null-order": "nulls-first"}]},
    {"order-id": 2, "fields": [{"source-id": 1, "transform": "identity", "direction": "desc", "null-order": "nulls-last"}]}
  ],
  "properties": {},
  "snapshots": [],
  "snapshot-log": [],
  "metadata-log": []
}`

func historyTable(t *testing.T) *table.Table {
	t.Helper()
	meta, err := table.ParseMetadataString(historyMetadata)
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}

	return table.New(table.Identifier{"db", "t"}, meta, "s3://bucket/t/metadata/v1.json", nil, nil)
}

func objectFromJSON[T any](t *testing.T, attrTypes func(T) types.Object, raw string) types.Object {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}

	return attrTypes(v)
}

func schemaObject(t *testing.T, raw string) types.Object {
	return objectFromJSON(t, func(s icebergTableSchema) types.Object {
		obj, d := types.ObjectValueFrom(context.Background(), icebergTableSchema{}.AttrTypes(), s)
		if d.HasError() {
			t.Fatalf("schema object: %v", d)
		}

		return obj
	}, raw)
}

func specObject(t *testing.T, raw string) types.Object {
	return objectFromJSON(t, func(s icebergTablePartitionSpec) types.Object {
		s.SpecID = types.Int64Unknown()
		obj, d := types.ObjectValueFrom(context.Background(), icebergTablePartitionSpec{}.AttrTypes(), s)
		if d.HasError() {
			t.Fatalf("spec object: %v", d)
		}

		return obj
	}, raw)
}

func sortObject(t *testing.T, raw string) types.Object {
	return objectFromJSON(t, func(s icebergTableSortOrder) types.Object {
		s.OrderID = types.Int64Unknown()
		obj, d := types.ObjectValueFrom(context.Background(), icebergTableSortOrder{}.AttrTypes(), s)
		if d.HasError() {
			t.Fatalf("sort order object: %v", d)
		}

		return obj
	}, raw)
}

func updateActions(t *testing.T, updates []table.Update) string {
	t.Helper()
	b, err := json.Marshal(updates)
	if err != nil {
		t.Fatalf("marshal updates: %v", err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal updates: %v", err)
	}
	out := ""
	for _, u := range raw {
		out += u["action"].(string)
		for _, k := range []string{"schema-id", "spec-id", "sort-order-id"} {
			if v, ok := u[k]; ok {
				b, _ := json.Marshal(v)
				out += "(" + string(b) + ")"
			}
		}
		out += ";"
	}

	return out
}

const (
	schemaV0 = `{"schema-id": 0, "fields": [
  {"id": 1, "name": "id", "required": true, "type": "long"},
  {"id": 2, "name": "ts", "required": false, "type": "timestamp"}]}`
	schemaV1 = `{"schema-id": 1, "fields": [
  {"id": 1, "name": "id", "required": true, "type": "long"},
  {"id": 2, "name": "ts", "required": false, "type": "timestamp"},
  {"id": 3, "name": "b", "required": false, "type": "string"}]}`
	schemaNew = `{"schema-id": 1, "fields": [
  {"id": 1, "name": "id", "required": true, "type": "long"},
  {"id": 2, "name": "ts", "required": false, "type": "timestamp"},
  {"id": 4, "name": "c", "required": false, "type": "string"}]}`
)

func TestCalculateSchemaUpdatesReusesExistingSchema(t *testing.T) {
	tbl := historyTable(t)
	r := &icebergTableResource{}

	cases := map[string]struct {
		plan string
		want string
	}{
		"unchanged":            {plan: schemaV1, want: ""},
		"revert to schema 0":   {plan: schemaV0, want: "set-current-schema(0);"},
		"genuinely new schema": {plan: schemaNew, want: "add-schema;set-current-schema(-1);"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			plan := &icebergTableResourceModel{Schema: schemaObject(t, tc.plan)}
			state := &icebergTableResourceModel{Schema: schemaObject(t, schemaV1)}
			got := updateActions(t, r.calculateSchemaUpdates(context.Background(), plan, state, tbl, &diags))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tc.want {
				t.Errorf("updates = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCalculatePartitionUpdatesReusesExistingSpec(t *testing.T) {
	tbl := historyTable(t)
	r := &icebergTableResource{}

	cases := map[string]struct {
		plan types.Object
		want string
	}{
		"unchanged": {
			plan: specObject(t, `{"fields": [{"source-ids": [2], "name": "ts_day", "transform": "day"}]}`),
			want: "",
		},
		"removed from config": {
			plan: types.ObjectUnknown(icebergTablePartitionSpec{}.AttrTypes()),
			want: "set-default-spec(0);",
		},
		"explicitly empty": {
			plan: specObject(t, `{"fields": []}`),
			want: "set-default-spec(0);",
		},
		"genuinely new spec": {
			plan: specObject(t, `{"fields": [{"source-ids": [1], "name": "id_b", "transform": "bucket[4]"}]}`),
			want: "add-spec;set-default-spec(-1);",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			plan := &icebergTableResourceModel{PartitionSpec: tc.plan}
			got := updateActions(t, r.calculatePartitionUpdates(context.Background(), plan, tbl, &diags))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tc.want {
				t.Errorf("updates = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCalculateSortOrderUpdatesReusesExistingOrder(t *testing.T) {
	tbl := historyTable(t)
	r := &icebergTableResource{}

	cases := map[string]struct {
		plan types.Object
		want string
	}{
		"unchanged with unknown order id": {
			plan: sortObject(t, `{"fields": [{"source-id": 1, "transform": "identity", "direction": "desc", "null-order": "nulls-last"}]}`),
			want: "",
		},
		"revert to order 1": {
			plan: sortObject(t, `{"fields": [{"source-id": 1, "transform": "identity", "direction": "asc", "null-order": "nulls-first"}]}`),
			want: "set-default-sort-order(1);",
		},
		"removed from config": {
			plan: types.ObjectUnknown(icebergTableSortOrder{}.AttrTypes()),
			want: "set-default-sort-order(0);",
		},
		"genuinely new order": {
			plan: sortObject(t, `{"fields": [{"source-id": 2, "transform": "identity", "direction": "asc", "null-order": "nulls-first"}]}`),
			want: "add-sort-order;set-default-sort-order(-1);",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			plan := &icebergTableResourceModel{SortOrder: tc.plan}
			got := updateActions(t, r.calculateSortOrderUpdates(context.Background(), plan, tbl, &diags))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tc.want {
				t.Errorf("updates = %q, want %q", got, tc.want)
			}
		})
	}
}
