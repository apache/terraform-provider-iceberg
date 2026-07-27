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
	"testing"

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
