---
page_title: "iceberg_polaris_catalog_role Resource - Iceberg"
subcategory: ""
description: |-
  A resource for managing Polaris catalog roles.
---

# iceberg_polaris_catalog_role (Resource)

A resource for managing Polaris catalog roles.

## Example Usage

```terraform
resource "iceberg_polaris_catalog_role" "namespace_admin" {
  catalog_name = "my_catalog"
  name         = "namespace_admin"
}
```

## Schema

### Required

- `catalog_name` (String) The catalog this role belongs to.
- `name` (String) The name of the catalog role.

### Optional

- `properties` (Map of String) Arbitrary metadata properties for the catalog role.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
terraform import iceberg_polaris_catalog_role.example catalog_name/role_name
```
