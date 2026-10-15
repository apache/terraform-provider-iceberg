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
- [iceberg_tables](data-sources/tables.md) — List table names in a namespace from the catalog.

## Resources

- [iceberg_namespace](resources/namespace.md) — Manage a catalog namespace.
- [iceberg_table](resources/table.md) — Manage an Iceberg table.

## Example Usage

Authenticate with a static bearer token:

```terraform
provider "iceberg" {
  catalog_uri = "https://catalog.example.com"
  token       = var.catalog_token
}
```

Authenticate with the OAuth2 client credentials flow. The provider fetches a
token from the token endpoint and refreshes it automatically:

```terraform
provider "iceberg" {
  catalog_uri = "https://catalog.example.com"

  auth = {
    oauth2 = {
      credential = "${var.client_id}:${var.client_secret}"
      scope      = "PRINCIPAL_ROLE:ALL"
    }
  }
}
```

`auth.oauth2` cannot be set together with `token` or `auth.sigv4`.

## Schema

### Required

- `catalog_uri` (String) The URI of the Iceberg REST catalog.

### Optional

- `auth` (Attributes) Authentication settings for the Iceberg REST catalog. (see [below for nested schema](#nestedatt--auth))
- `headers` (Map of String, Sensitive) The headers to use for authentication. With `auth.oauth2`, an `Authorization` entry is not sent, and the other headers are also sent to the OAuth2 token endpoint.
- `token` (String, Sensitive) The token to use for authentication.
- `type` (String) The type of catalog. Use 'rest' for a plain REST catalog.
- `warehouse` (String) The warehouse to use for the Iceberg REST catalog. This will be passed as `warehouse` property in the catalog properties.

<a id="nestedatt--auth"></a>
### Nested Schema for `auth`

Optional:

- `oauth2` (Attributes) Authenticate with the OAuth2 client credentials flow. Tokens are fetched and refreshed automatically. (see [below for nested schema](#nestedatt--auth--oauth2))
- `sigv4` (Attributes) Sign requests with AWS Signature Version 4, as catalogs such as AWS Glue require. Setting this attribute enables signing. (see [below for nested schema](#nestedatt--auth--sigv4))

<a id="nestedatt--auth--oauth2"></a>
### Nested Schema for `auth.oauth2`

Required:

- `credential` (String, Sensitive) The client credential, formatted as `client_id:client_secret`. A value without a colon is used as the client secret with an empty client ID.

Optional:

- `audience` (String) The audience to request.
- `resource` (String) The resource to request.
- `scope` (String) The scope to request. Defaults to `catalog`.
- `server_uri` (String) The OAuth2 token endpoint. Defaults to `{catalog_uri}/v1/oauth/tokens`.

<a id="nestedatt--auth--sigv4"></a>
### Nested Schema for `auth.sigv4`

Optional:

- `access_key_id` (String, Sensitive) Access key ID. When omitted, the standard AWS credential chain (environment, shared config, instance role) is used.
- `region` (String) Signing region. When omitted, the region from the AWS environment (`AWS_REGION`, shared config) is used.
- `secret_access_key` (String, Sensitive) Secret access key. Must be set together with `access_key_id`.
- `session_token` (String, Sensitive) Session token for temporary (STS) credentials. Requires `access_key_id` and `secret_access_key`.
- `signing_name` (String) Signing service name (the credential-scope service). Defaults to `execute-api`. Use `glue` for AWS Glue.

## AWS SigV4 authentication

Some REST catalogs authenticate requests with AWS Signature Version 4 instead of
a bearer token. Setting `auth.sigv4` signs every request. The signing region
comes from `region` or, when that is omitted, from the AWS environment; the
provider reports an error when neither sets one. The signing name defaults to
`execute-api`; override it for catalogs that scope signatures to a different
service.

Credentials come from `access_key_id` and `secret_access_key`, plus
`session_token` for temporary credentials, when they are set, and from the
standard AWS credential chain otherwise. Values from `headers` are set before
signing, so they are part of the signature.

When `token`, or an `Authorization` entry in `headers`, is also set, SigV4 keeps
the `Authorization` header and that value is sent as `Original-Authorization`,
matching the Java client, for catalogs that sit behind a SigV4 gateway and
authenticate with OAuth themselves.

```terraform
# AWS Glue REST catalog
provider "iceberg" {
  catalog_uri = "https://glue.us-east-1.amazonaws.com/iceberg"
  warehouse   = "123456789012"

  auth = {
    sigv4 = {
      region       = "us-east-1"
      signing_name = "glue"
      # Omit the keys to use the standard AWS credential chain.
    }
  }
}
```
