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
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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
				// Import fills both halves of each source pair, matching state.
				ResourceName:      "iceberg_table.test",
				ImportState:       true,
				ImportStateVerify: true,
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

func TestAccIcebergTablePartitionSortBySourceName(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "by_source_name_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// No column has an explicit id; every source is referenced by name.
				Config: testAccIcebergTableBySourceNameConfig(providerCfg, tableName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.struct_properties.fields.0.id", "4"),

					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_names.#", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_names.0", "event_time"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.transform", "day"),

					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_name", "id"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.1.source_id", "4"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.1.source_name", "location.lat"),
				),
			},
			{
				Config:   testAccIcebergTableBySourceNameConfig(providerCfg, tableName, false),
				PlanOnly: true,
			},
			{
				// Import fills both halves of each source pair, matching state.
				ResourceName:      "iceberg_table.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// A new column, plus a partition field and a sort field naming it, in one apply.
				Config: testAccIcebergTableBySourceNameConfig(providerCfg, tableName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.#", "4"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.3.name", "region"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.3.id", "5"),

					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_names.0", "event_time"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.1.source_ids.0", "5"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.1.source_names.0", "region"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.1.name", "region_identity"),

					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "3"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.2.source_id", "5"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.2.source_name", "region"),
				),
			},
			{
				Config:   testAccIcebergTableBySourceNameConfig(providerCfg, tableName, true),
				PlanOnly: true,
			},
		},
	})
}

func testAccIcebergTableBySourceNameConfig(providerCfg, tableName string, withRegion bool) string {
	regionColumn, regionPartition, regionSort := "", "", ""
	if withRegion {
		regionColumn = `,
      {
        name     = "region"
        type     = "string"
        required = false
      }`
		regionPartition = `,
      {
        source_names = ["region"]
        name         = "region_identity"
        transform    = "identity"
      }`
		regionSort = `,
      {
        source_name = "region"
        transform   = "identity"
        direction   = "asc"
        null_order  = "nulls-last"
      }`
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_by_name" {
  name = ["db_by_name"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_by_name.name
  name      = "%s"
  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      },
      {
        name     = "event_time"
        type     = "timestamptz"
        required = false
      },
      {
        name     = "location"
        type     = "struct"
        required = false
        struct_properties = {
          fields = [
            {
              name     = "lat"
              type     = "double"
              required = false
            }
          ]
        }
      }%s
    ]
  }
  partition_spec = {
    fields = [
      {
        source_names = ["event_time"]
        name         = "event_time_day"
        transform    = "day"
      }%s
    ]
  }
  sort_order = {
    fields = [
      {
        source_name = "id"
        transform   = "identity"
        direction   = "asc"
        null_order  = "nulls-first"
      },
      {
        source_name = "location.lat"
        transform   = "identity"
        direction   = "desc"
        null_order  = "nulls-last"
      }%s
    ]
  }
}
`, tableName, regionColumn, regionPartition, regionSort)
}

func TestAccIcebergTableRenameSourceColumn(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "rename_source_table"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableRenameSourceConfig(providerCfg, tableName, "ts"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.spec_id", "0"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_names.0", "ts"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.order_id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_name", "ts"),
				),
			},
			{
				// Renaming the column (same id) and its references together
				// leaves the partition spec and sort order as they were.
				Config: testAccIcebergTableRenameSourceConfig(providerCfg, tableName, "event_ts"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.name", "event_ts"),
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.spec_id", "0"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.field_id", "1000"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_names.0", "event_ts"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.order_id", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_id", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_name", "event_ts"),
				),
			},
			{
				Config:   testAccIcebergTableRenameSourceConfig(providerCfg, tableName, "event_ts"),
				PlanOnly: true,
			},
		},
	})
}

func testAccIcebergTableRenameSourceConfig(providerCfg, tableName, tsName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_rename_source" {
  name = ["db_rename_source"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_rename_source.name
  name      = "%[1]s"
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
        name     = "%[2]s"
        type     = "timestamptz"
        required = false
      }
    ]
  }
  partition_spec = {
    fields = [
      {
        source_names = ["%[2]s"]
        name         = "ts_day"
        transform    = "day"
      }
    ]
  }
  sort_order = {
    fields = [
      {
        source_name = "%[2]s"
        transform   = "identity"
        direction   = "asc"
        null_order  = "nulls-first"
      }
    ]
  }
}
`, tableName, tsName)
}

func TestAccIcebergTableSourceReferenceValidation(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "source_validation_table"
	both := regexp.MustCompile(`(?s)Invalid Attribute Combination.*2 attributes specified when one \(and only one\) of`)
	neither := regexp.MustCompile(`(?s)Invalid Attribute Combination.*No attribute specified when one \(and only one\) of`)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccIcebergTableSourceRefConfig(providerCfg, tableName, `source_ids = [1], source_names = ["id"]`, `source_name = "id"`),
				ExpectError: both,
			},
			{
				Config:      testAccIcebergTableSourceRefConfig(providerCfg, tableName, ``, `source_name = "id"`),
				ExpectError: neither,
			},
			{
				Config:      testAccIcebergTableSourceRefConfig(providerCfg, tableName, `source_names = ["id"]`, `source_id = 1, source_name = "id"`),
				ExpectError: both,
			},
			{
				Config:      testAccIcebergTableSourceRefConfig(providerCfg, tableName, `source_names = ["id"]`, ``),
				ExpectError: neither,
			},
			{
				Config:      testAccIcebergTableSourceRefConfig(providerCfg, tableName, `source_names = ["missing"]`, `source_name = "id"`),
				ExpectError: regexp.MustCompile(`(?s)failed to convert partition spec.*partition field "p".*source column "missing" not found`),
			},
			{
				Config:      testAccIcebergTableSourceRefConfig(providerCfg, tableName, `source_names = ["id"]`, `source_name = "ID"`),
				ExpectError: regexp.MustCompile(`(?s)failed to convert sort order.*source column "ID" not found`),
			},
		},
	})
}

func testAccIcebergTableSourceRefConfig(providerCfg, tableName, partitionRef, sortRef string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_source_validation" {
  name = ["db_source_validation"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_source_validation.name
  name      = "%s"
  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
  partition_spec = {
    fields = [
      {
        %s
        name      = "p"
        transform = "identity"
      }
    ]
  }
  sort_order = {
    fields = [
      {
        %s
        transform  = "identity"
        direction  = "asc"
        null_order = "nulls-first"
      }
    ]
  }
}
`, tableName, partitionRef, sortRef)
}

// TestAccIcebergTableMigrateSourceIDsToNames covers moving an existing table
// from explicit ids to names: the switch itself must be a no-op.
func TestAccIcebergTableMigrateSourceIDsToNames(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "migrate_to_names_table"

	unchangedIDs := resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
		resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.0.id", "1"),
		resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.1.id", "2"),
		resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.id", "3"),
		resource.TestCheckResourceAttr("iceberg_table.test", "schema.fields.2.struct_properties.fields.0.id", "4"),
		resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "1"),
		resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_names.0", "snapshot_date"),
		resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_id", "1"),
		resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.0.source_name", "snapshot_date"),
		resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.1.source_id", "2"),
		resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.1.source_name", "market_code"),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableMigrateConfig(providerCfg, tableName, false, false),
				Check:  unchangedIDs,
			},
			{
				Config: testAccIcebergTableMigrateConfig(providerCfg, tableName, true, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: unchangedIDs,
			},
			{
				// Partition by a nested struct field, by name.
				Config: testAccIcebergTableMigrateConfig(providerCfg, tableName, true, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_table.test", "schema.id", "0"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "2"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.0.source_ids.0", "1"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.1.source_ids.0", "4"),
					resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.1.source_names.0", "collection_info.source"),
				),
			},
		},
	})
}

func testAccIcebergTableMigrateConfig(providerCfg, tableName string, byName, nestedPartition bool) string {
	id := func(n int) string {
		if byName {
			return ""
		}

		return fmt.Sprintf("id = %d\n", n)
	}
	partitionRef, sortRef1, sortRef2 := `source_ids = [1]`, `source_id = 1`, `source_id = 2`
	if byName {
		partitionRef, sortRef1, sortRef2 = `source_names = ["snapshot_date"]`, `source_name = "snapshot_date"`, `source_name = "market_code"`
	}
	nested := ""
	if nestedPartition {
		nested = `,
      {
        source_names = ["collection_info.source"]
        name         = "collection_source"
        transform    = "identity"
      }`
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_migrate" {
  name = ["db_migrate"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_migrate.name
  name      = "%s"
  schema = {
    fields = [
      {
        %s
        name     = "snapshot_date"
        type     = "date"
        required = true
      },
      {
        %s
        name     = "market_code"
        type     = "string"
        required = false
      },
      {
        %s
        name     = "collection_info"
        type     = "struct"
        required = false
        struct_properties = {
          fields = [
            {
              %s
              name     = "source"
              type     = "string"
              required = false
            }
          ]
        }
      }
    ]
  }
  partition_spec = {
    fields = [
      {
        %s
        name      = "snapshot_date_identity"
        transform = "identity"
      }%s
    ]
  }
  sort_order = {
    fields = [
      {
        %s
        transform  = "identity"
        direction  = "asc"
        null_order = "nulls-first"
      },
      {
        %s
        transform  = "identity"
        direction  = "asc"
        null_order = "nulls-first"
      }
    ]
  }
}
`, tableName, id(1), id(2), id(3), id(4), partitionRef, nested, sortRef1, sortRef2)
}

// TestAccIcebergTableSourceFieldListChanges removes, inserts and reorders
// partition and sort fields that mix id and name references. Terraform matches
// list elements by position, so a field that shifts must still end up with its
// own ids and names rather than its neighbour's.
func TestAccIcebergTableSourceFieldListChanges(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)
	tableName := "source_field_list_table"

	partTS := `{ source_names = ["ts"], name = "ts_day", transform = "day" }`
	partRegion := `{ source_ids = [3], name = "region_identity", transform = "identity" }`
	partCategory := `{ source_names = ["category"], name = "category_bucket", transform = "bucket[4]" }`
	partID := `{ source_ids = [1], name = "id_bucket", transform = "bucket[8]" }`

	sortID := `{ source_id = 1, transform = "identity", direction = "asc", null_order = "nulls-first" }`
	sortRegion := `{ source_name = "region", transform = "identity", direction = "desc", null_order = "nulls-last" }`
	sortCategory := `{ source_id = 4, transform = "identity", direction = "asc", null_order = "nulls-last" }`
	sortTS := `{ source_name = "ts", transform = "identity", direction = "asc", null_order = "nulls-first" }`

	// Each step leaves a spec and sort order the table has not had before.
	steps := []struct {
		partitions, sorts []string
		check             resource.TestCheckFunc
	}{
		{
			partitions: []string{partTS, partRegion, partCategory},
			sorts:      []string{sortID, sortRegion, sortCategory},
			check: resource.ComposeTestCheckFunc(
				checkPartitionSource(0, "ts_day", 2, "ts"),
				checkPartitionSource(1, "region_identity", 3, "region"),
				checkPartitionSource(2, "category_bucket", 4, "category"),
				checkSortSource(0, 1, "id"),
				checkSortSource(1, 3, "region"),
				checkSortSource(2, 4, "category"),
			),
		},
		{
			// Remove the middle field.
			partitions: []string{partTS, partCategory},
			sorts:      []string{sortID, sortCategory},
			check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "2"),
				checkPartitionSource(0, "ts_day", 2, "ts"),
				checkPartitionSource(1, "category_bucket", 4, "category"),
				resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "2"),
				checkSortSource(0, 1, "id"),
				checkSortSource(1, 4, "category"),
			),
		},
		{
			// Remove the first field.
			partitions: []string{partCategory},
			sorts:      []string{sortCategory},
			check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("iceberg_table.test", "partition_spec.fields.#", "1"),
				checkPartitionSource(0, "category_bucket", 4, "category"),
				resource.TestCheckResourceAttr("iceberg_table.test", "sort_order.fields.#", "1"),
				checkSortSource(0, 4, "category"),
			),
		},
		{
			// Insert a field at the beginning.
			partitions: []string{partID, partCategory},
			sorts:      []string{sortTS, sortCategory},
			check: resource.ComposeTestCheckFunc(
				checkPartitionSource(0, "id_bucket", 1, "id"),
				checkPartitionSource(1, "category_bucket", 4, "category"),
				checkSortSource(0, 2, "ts"),
				checkSortSource(1, 4, "category"),
			),
		},
		{
			// Reorder.
			partitions: []string{partCategory, partID},
			sorts:      []string{sortCategory, sortTS},
			check: resource.ComposeTestCheckFunc(
				checkPartitionSource(0, "category_bucket", 4, "category"),
				checkPartitionSource(1, "id_bucket", 1, "id"),
				checkSortSource(0, 4, "category"),
				checkSortSource(1, 2, "ts"),
			),
		},
	}

	var testSteps []resource.TestStep
	for _, step := range steps {
		config := testAccIcebergTableSourceFieldsConfig(providerCfg, tableName, step.partitions, step.sorts)
		testSteps = append(testSteps,
			resource.TestStep{Config: config, Check: step.check},
			resource.TestStep{Config: config, PlanOnly: true},
		)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    testSteps,
	})
}

func checkPartitionSource(index int, name string, sourceID int, sourceName string) resource.TestCheckFunc {
	prefix := fmt.Sprintf("partition_spec.fields.%d.", index)

	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("iceberg_table.test", prefix+"name", name),
		resource.TestCheckResourceAttr("iceberg_table.test", prefix+"source_ids.0", strconv.Itoa(sourceID)),
		resource.TestCheckResourceAttr("iceberg_table.test", prefix+"source_names.0", sourceName),
	)
}

func checkSortSource(index int, sourceID int, sourceName string) resource.TestCheckFunc {
	prefix := fmt.Sprintf("sort_order.fields.%d.", index)

	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("iceberg_table.test", prefix+"source_id", strconv.Itoa(sourceID)),
		resource.TestCheckResourceAttr("iceberg_table.test", prefix+"source_name", sourceName),
	)
}

func testAccIcebergTableSourceFieldsConfig(providerCfg, tableName string, partitions, sorts []string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db_source_fields" {
  name = ["db_source_fields"]
}

resource "iceberg_table" "test" {
  namespace = iceberg_namespace.db_source_fields.name
  name      = "%s"
  schema = {
    fields = [
      { id = 1, name = "id", type = "long", required = true },
      { id = 2, name = "ts", type = "timestamptz", required = false },
      { id = 3, name = "region", type = "string", required = false },
      { id = 4, name = "category", type = "string", required = false },
    ]
  }
  partition_spec = {
    fields = [
      %s
    ]
  }
  sort_order = {
    fields = [
      %s
    ]
  }
}
`, tableName, strings.Join(partitions, ",\n      "), strings.Join(sorts, ",\n      "))
}
