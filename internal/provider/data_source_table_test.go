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

// TestAccIcebergTableDataSource_Full runs end-to-end against a real REST catalog when ICEBERG_CATALOG_URI is set.
func TestAccIcebergTableDataSource_Full(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("ICEBERG_CATALOG_URI not set, skipping table data source E2E test")
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergTableDataSourceBasicConfig(providerCfg, "ds_e2e_basic"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.iceberg_table.read", "namespace.0", "db_table_ds_basic"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "name", "ds_e2e_basic"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "schema.fields.0.name", "id"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "schema.fields.0.type", "long"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "schema.fields.1.name", "data"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "schema.fields.1.type", "string"),
					resource.TestCheckResourceAttrPair("data.iceberg_table.read", "id", "iceberg_table.subject", "id"),
					resource.TestCheckResourceAttrSet("data.iceberg_table.read", "schema.id"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "partition_spec.fields.#", "0"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "sort_order.fields.#", "0"),
				),
			},
			{
				Config: testAccIcebergTableDataSourcePartitionSortConfig(providerCfg, "ds_e2e_partition_sort", "bucket[16]", "asc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.iceberg_table.read", "name", "ds_e2e_partition_sort"),
					resource.TestCheckResourceAttrPair("data.iceberg_table.read", "id", "iceberg_table.subject", "id"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "partition_spec.fields.#", "1"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "partition_spec.fields.0.source_ids.0", "1"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "partition_spec.fields.0.field_id", "1000"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "partition_spec.fields.0.name", "id_bucket"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "partition_spec.fields.0.transform", "bucket[16]"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "sort_order.fields.#", "1"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "sort_order.fields.0.source_id", "1"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "sort_order.fields.0.transform", "identity"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "sort_order.fields.0.direction", "asc"),
					resource.TestCheckResourceAttr("data.iceberg_table.read", "sort_order.fields.0.null_order", "nulls-first"),
				),
			},
		},
	})
}

// TestAccIcebergTableDataSource_TableNotFound expects a clear error when the table does not exist.
func TestAccIcebergTableDataSource_TableNotFound(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("ICEBERG_CATALOG_URI not set, skipping table data source E2E test")
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccIcebergTableDataSourceMissingTableConfig(providerCfg),
				ExpectError: regexp.MustCompile(`Table not found`),
			},
		},
	})
}

func testAccIcebergTableDataSourceBasicConfig(providerCfg string, tableName string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db" {
  name = ["db_table_ds_basic"]
}

resource "iceberg_table" "subject" {
  namespace = iceberg_namespace.db.name
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

data "iceberg_table" "read" {
  namespace = iceberg_namespace.db.name
  name      = iceberg_table.subject.name
}
`, tableName)
}

func testAccIcebergTableDataSourcePartitionSortConfig(providerCfg string, tableName string, partitionTransform string, sortDirection string) string {
	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "db" {
  name = ["db_table_ds_part"]
}

resource "iceberg_table" "subject" {
  namespace = iceberg_namespace.db.name
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

data "iceberg_table" "read" {
  namespace = iceberg_namespace.db.name
  name      = iceberg_table.subject.name
}
`, tableName, partitionTransform, sortDirection)
}

func testAccIcebergTableDataSourceMissingTableConfig(providerCfg string) string {
	return providerCfg + `
resource "iceberg_namespace" "empty" {
  name = ["db_table_ds_missing"]
}

data "iceberg_table" "missing" {
  namespace = iceberg_namespace.empty.name
  name      = "definitely_no_such_table"
}
`
}
