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

- [iceberg_table](data-sources/table.md) — Read metadata for an existing table from the catalog.
- [iceberg_polaris_principal_role](data-sources/polaris_principal_role.md) — Look up a Polaris principal role by name.
- [iceberg_polaris_catalog_role](data-sources/polaris_catalog_role.md) — Look up a Polaris catalog role by name and catalog.

## Resources

- [iceberg_namespace](resources/namespace.md) — Manage a catalog namespace.
- [iceberg_table](resources/table.md) — Manage an Iceberg table.
- [iceberg_polaris_principal](resources/polaris_principal.md) — Manage a Polaris principal (Polaris deployments).
- [iceberg_polaris_principal_role](resources/polaris_principal_role.md) — Manage a Polaris principal role.
- [iceberg_polaris_principal_role_assignment](resources/polaris_principal_role_assignment.md) — Assign a principal role to a Polaris principal.
- [iceberg_polaris_catalog_role](resources/polaris_catalog_role.md) — Manage a Polaris catalog role.
- [iceberg_polaris_catalog_role_assignment](resources/polaris_catalog_role_assignment.md) — Assign a catalog role to a principal role.
- [iceberg_polaris_grant](resources/polaris_grant.md) — Grant a privilege on a securable object to a catalog role.

## Schema

### Required

- `catalog_uri` (String) The URI of the Iceberg REST catalog.

### Optional

- `headers` (Map of String, Sensitive) The headers to use for authentication.
- `polaris_settings` (Block, Optional) Settings specific to Polaris when type = 'polaris'. (see [below for nested schema](#nestedblock--polaris_settings))
- `token` (String, Sensitive) The token to use for authentication.
- `type` (String) The type of catalog. Use 'rest' for a plain REST catalog, or 'polaris' for Polaris (REST catalog with Polaris management).
- `warehouse` (String) The warehouse to use for the Iceberg REST catalog. This will be passed as `warehouse` property in the catalog properties.

<a id="nestedblock--polaris_settings"></a>
### Nested Schema for `polaris_settings`

Optional:

- `catalog_name` (String) Default Polaris catalog name for RBAC resources.
- `management_uri` (String) The base URI for the Polaris Management API. If omitted, it will be derived from catalog_uri by appending '/api/management/v1'.
