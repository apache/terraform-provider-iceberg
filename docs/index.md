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

- `headers` (Map of String, Sensitive) The headers to use for authentication.
- `sigv4_access_key_id` (String, Sensitive) Access key ID for SigV4 signing. When omitted, the standard AWS credential chain (environment, shared config, instance role) is used.
- `sigv4_enabled` (Boolean) Enable AWS SigV4 request signing for the REST catalog. Required by catalogs that authenticate with SigV4, such as AWS Glue.
- `sigv4_region` (String) Signing region for SigV4. When omitted, the region from the AWS environment (`AWS_REGION`, shared config) is used.
- `sigv4_secret_access_key` (String, Sensitive) Secret access key for SigV4 signing. Must be paired with `sigv4_access_key_id`.
- `sigv4_session_token` (String, Sensitive) Optional session token for temporary (STS) SigV4 credentials.
- `sigv4_signing_name` (String) Signing service name for SigV4 (the credential-scope service). Defaults to `execute-api`. Use `glue` for AWS Glue.
- `token` (String, Sensitive) The token to use for authentication.
- `type` (String) The type of catalog. Use 'rest' for a plain REST catalog.
- `warehouse` (String) The warehouse to use for the Iceberg REST catalog. This will be passed as `warehouse` property in the catalog properties.

## AWS SigV4 authentication

Some REST catalogs authenticate requests with AWS Signature Version 4 instead of
a bearer token. Set `sigv4_enabled = true` and, unless the AWS environment
provides one, the signing region. The signing name defaults to `execute-api`;
override it for catalogs that scope signatures to a different service.

When `token` is also set, SigV4 keeps the `Authorization` header and the bearer
token is sent as `Original-Authorization`, matching the Java client, for catalogs
that sit behind a SigV4 gateway and authenticate with OAuth themselves.

```terraform
# AWS Glue REST catalog
provider "iceberg" {
  catalog_uri        = "https://glue.us-east-1.amazonaws.com/iceberg"
  warehouse          = "123456789012"
  sigv4_enabled      = true
  sigv4_region       = "us-east-1"
  sigv4_signing_name = "glue"
  # Credentials resolved from the standard AWS chain when the keys are omitted,
  # or set sigv4_access_key_id / sigv4_secret_access_key explicitly.
}
```
