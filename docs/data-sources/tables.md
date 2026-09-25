---
page_title: "iceberg_tables Data Source - Iceberg"
subcategory: ""
description: |-
  Lists table names in an Iceberg namespace from the catalog.
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

# iceberg_tables (Data Source)

Lists table names in an Iceberg namespace from the catalog.

Use this data source to discover existing tables in a namespace—for example, to validate namespace contents, wire downstream jobs, or drive `for_each` over tables created outside Terraform.

- `tables` returns table names only (e.g. `events`, `orders`).
- `identifiers` returns dot-separated full table identifiers (e.g. `analytics.raw.events`), matching the `id` format used by [`iceberg_table`](table.md).

Listing is non-recursive: only tables directly in the given namespace are returned, not tables in child namespaces.

## Example Usage

### List tables in a namespace

```terraform
data "iceberg_tables" "analytics" {
  namespace = ["analytics", "raw"]
}

output "table_names" {
  value = data.iceberg_tables.analytics.tables
}

output "table_identifiers" {
  value = data.iceberg_tables.analytics.identifiers
}
```

### Drive resources with `for_each`

Use `tables` when the namespace is already known (short names). Use `identifiers` when downstream systems need the full catalog path.

```terraform
data "iceberg_tables" "in_ns" {
  namespace = ["analytics", "raw"]
}

resource "example_downstream" "per_table" {
  for_each = toset(data.iceberg_tables.in_ns.tables)

  table_name = each.value
}

output "full_table_paths" {
  value = data.iceberg_tables.in_ns.identifiers
}
```

### List tables after creating them in the same configuration

When tables are managed in the same Terraform configuration, add `depends_on` so the data source is read after those tables exist. Referencing only `namespace` creates an implicit dependency on the namespace resource, not on individual tables.

```terraform
resource "iceberg_namespace" "example" {
  name = ["example_namespace"]
}

resource "iceberg_table" "events" {
  namespace = iceberg_namespace.example.name
  name      = "events"

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

resource "iceberg_table" "orders" {
  namespace = iceberg_namespace.example.name
  name      = "orders"

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

data "iceberg_tables" "example" {
  namespace = iceberg_namespace.example.name

  depends_on = [
    iceberg_table.events,
    iceberg_table.orders,
  ]
}
```

## Schema

### Required

- `namespace` (List of String) The namespace to list tables in.

### Read-Only

- `id` (String) Dot-separated full namespace identifier.
- `tables` (List of String) Table names in the namespace, without namespace segments. Sorted alphabetically.
- `identifiers` (List of String) Dot-separated full table identifiers (namespace + table name), e.g. `analytics.raw.events`. Sorted alphabetically. Matches the `id` format of [`iceberg_table`](table.md).

## Error handling

- If the namespace does not exist, apply fails with a **Namespace not found** error.
- If listing fails for another reason (including a mid-pagination catalog error), apply fails with **failed to list tables** and the underlying catalog message.
- If `catalog_uri` is not configured on the provider, apply fails with **Catalog not available**.
