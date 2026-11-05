---
page_title: "iceberg_table Data Source - Iceberg"
subcategory: ""
description: |-
  Reads metadata for an existing Iceberg table from the catalog.
---

<!--
  - Licensed to the Apache Software Foundation (ASF) under one
  - or more contributor license agreements.  See the NOTICE file
  - distributed with this work for additional information
  - regarding copyright ownership.  The ASF licenses this file
  - to you under the Apache License, Version 2.0 (the
  - "License"); you may not use this file except in compliance
  - with the License.  You may obtain a copy of the License at
  -
  -   http://www.apache.org/licenses/LICENSE-2.0
  -
  - Unless required by applicable law or agreed to in writing,
  - software distributed under the License is distributed on an
  - "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
  - KIND, either express or implied.  See the License for the
  - specific language governing permissions and limitations
  - under the License.
  -->

# iceberg_table (Data Source)

Reads metadata for an existing Iceberg table from the catalog.

Use this data source to inspect tables created outside Terraform, to reference tables managed in another workspace, or to read the current catalog state of a table also managed by an [`iceberg_table` resource](../resources/table.md).

## Example Usage

### Read a table by namespace and name

```terraform
data "iceberg_table" "events" {
  namespace = ["analytics", "prod"]
  name      = "events"
}

output "events_table_id" {
  value = data.iceberg_table.events.id
}

output "events_id_column" {
  value = data.iceberg_table.events.schema.fields[0].name
}
```

### Read a table managed by this provider

```terraform
resource "iceberg_namespace" "example" {
  name = ["example_namespace"]
}

resource "iceberg_table" "example" {
  namespace = iceberg_namespace.example.name
  name      = "example_table"

  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
}

data "iceberg_table" "example" {
  depends_on = [iceberg_table.example]

  namespace = iceberg_namespace.example.name
  name      = iceberg_table.example.name
}
```

## Schema

### Required

- `name` (String) The name of the table.
- `namespace` (List of String) The namespace of the table.

### Read-Only

- `id` (String) Dot-separated full table identifier (namespace segments and table name).
- `partition_spec` (Attributes) The partition spec of the table. Empty when the table is not partitioned. Nested field shapes match the [`iceberg_table` resource](../resources/table.md#nestedatt--partition_spec), `source_names` holds the column names of `source_ids` in the current schema, or null if a source column is no longer in it.
- `schema` (Attributes) The current schema of the table. Nested field shapes match the [`iceberg_table` resource](../resources/table.md#nestedatt--schema); all nested attributes are computed on read.
- `server_properties` (Map of String) Properties from table metadata as returned by the catalog (the same values surfaced as `server_properties` on the table resource after apply).
- `sort_order` (Attributes) The sort order of the table. Empty when the table is unsorted. Nested field shapes match the [`iceberg_table` resource](../resources/table.md#nestedatt--sort_order), `source_name` holds the column name of `source_id` in the current schema, or null if the source column is no longer in it.

## Error handling

- If the table does not exist, apply fails with a **Table not found** error.
- If `catalog_uri` is not configured on the provider, apply fails with **Catalog not available**.
