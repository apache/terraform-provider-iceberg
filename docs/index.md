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

- `auth` (Attributes) Authentication settings for the Iceberg REST catalog. (see [below for nested schema](#nestedatt--auth))
- `headers` (Map of String, Sensitive) The headers to use for authentication.
- `token` (String, Sensitive) The token to use for authentication.
- `type` (String) The type of catalog. Use 'rest' for a plain REST catalog.
- `warehouse` (String) The warehouse to use for the Iceberg REST catalog. This will be passed as `warehouse` property in the catalog properties.

<a id="nestedatt--auth"></a>
### Nested Schema for `auth`

Optional:

- `sigv4` (Attributes) Sign requests with AWS Signature Version 4, as catalogs such as AWS Glue require. Setting this attribute enables signing. (see [below for nested schema](#nestedatt--auth--sigv4))

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
