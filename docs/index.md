---
page_title: "Iceberg Provider"
description: |-
  Use Terraform to interact with Iceberg REST Catalog instances.
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

# Iceberg Provider

Use Terraform to interact with Iceberg REST Catalog instances.

## Example Usage

The provider connects to an Iceberg REST catalog. A basic configuration can
create a namespace and a table in that namespace:

```terraform
provider "iceberg" {
  catalog_uri = "http://localhost:8181"
}

resource "iceberg_namespace" "analytics" {
  name = ["analytics"]

  user_properties = {
    owner = "data-platform"
  }
}

resource "iceberg_table" "events" {
  namespace = iceberg_namespace.analytics.name
  name      = "events"

  schema = {
    fields = [
      {
        name     = "event_id"
        type     = "long"
        required = true
      },
      {
        name     = "event_type"
        type     = "string"
        required = true
      },
      {
        name     = "event_time"
        type     = "timestamp"
        required = true
      }
    ]
  }

  user_properties = {
    "write.format.default" = "parquet"
  }
}
```

Namespace names are lists of identifier segments. A catalog that supports
nested namespaces can use a value such as `["analytics", "production"]`.
Tables take the namespace as a list and the table name separately, so referring
to `iceberg_namespace.analytics.name` keeps the relationship explicit in the
Terraform graph.

See the [`iceberg_namespace`](resources/namespace.md) and
[`iceberg_table`](resources/table.md) resource pages for the complete schemas,
imports, schema evolution, partition specifications, and sort orders.

## Provider Configuration

### Catalog URI

`catalog_uri` is the base address of the Iceberg REST catalog deployment. For
example, the development catalog used by this repository is
`http://localhost:8181`.

The Iceberg Go client appends the REST API version path itself and first requests
`/v1/config`. Do not add `/v1` to `catalog_uri`. If the catalog is deployed below
a base path, include that deployment path in the URI; for example,
`https://catalog.example.com/iceberg`.

### REST prefix

The Iceberg REST configuration response can provide a `prefix`. That prefix is a
server-selected routing path used for subsequent catalog requests after
`/v1/config` is loaded. The Terraform provider does not expose a `prefix`
argument: the Iceberg Go client reads and applies a prefix returned by the
catalog automatically.

In other words, configure the catalog deployment URI in `catalog_uri`, not the
server's REST prefix.

### Catalog type

The provider currently supports Iceberg REST catalogs only. The optional `type`
argument defaults to `rest`; setting any other value returns an unsupported
catalog type error.

```terraform
provider "iceberg" {
  catalog_uri = "https://catalog.example.com"
  type        = "rest"
}
```

Because `rest` is the default, `type` normally does not need to be set.

### Warehouse

Some REST catalog deployments use a warehouse identifier to select the catalog
or storage namespace. Set `warehouse` when your catalog requires one. The value
is sent to the REST catalog during configuration; its expected format is defined
by the catalog server.

```terraform
provider "iceberg" {
  catalog_uri = "https://catalog.example.com"
  warehouse   = "analytics"
}
```

## Authentication

The provider currently supports a static OAuth bearer token and custom request
headers. Both `token` and `headers` are marked sensitive by the provider.

Keep credentials out of checked-in Terraform files. Sensitive values can still
be present in Terraform state, so protect state storage according to your
Terraform deployment's security requirements.

### OAuth bearer token

Set `token` to a bearer token accepted by the Iceberg REST catalog:

```terraform
variable "iceberg_token" {
  type      = string
  sensitive = true
}

provider "iceberg" {
  catalog_uri = "https://catalog.example.com"
  token       = var.iceberg_token
}
```

For local use, Terraform can receive the variable from the environment without
putting the token in a `.tf` file:

```shell
export TF_VAR_iceberg_token="..."
terraform plan
```

### Custom headers

Use `headers` for catalogs that authenticate with an API key or another
catalog-specific HTTP header, or that require additional routing headers:

```terraform
variable "iceberg_headers" {
  type      = map(string)
  sensitive = true
}

provider "iceberg" {
  catalog_uri = "https://catalog.example.com"
  headers     = var.iceberg_headers
}
```

For example, the calling module can supply a sensitive map containing an API-key
header. Avoid configuring the same `Authorization` header through both `token`
and `headers`; choose one mechanism for authorization so the request does not
carry duplicate credentials.

## Data Sources

- [iceberg_namespace](data-sources/namespace.md) — Read metadata for an existing namespace from the catalog.
- [iceberg_table](data-sources/table.md) — Read metadata for an existing table from the catalog.

## Resources

- [iceberg_namespace](resources/namespace.md) — Manage a catalog namespace.
- [iceberg_table](resources/table.md) — Manage an Iceberg table.

## Schema

### Required

- `catalog_uri` (String) The URI of the Iceberg REST catalog.

### Optional

- `headers` (Map of String, Sensitive) The headers to use for authentication.
- `token` (String, Sensitive) The token to use for authentication.
- `type` (String) The type of catalog. Use 'rest' for a plain REST catalog.
- `warehouse` (String) The warehouse to use for the Iceberg REST catalog. This will be passed as `warehouse` property in the catalog properties.
