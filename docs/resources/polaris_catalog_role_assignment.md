---
page_title: "iceberg_polaris_catalog_role_assignment Resource - Iceberg"
subcategory: ""
description: |-
  Assigns a catalog role to a principal role.
---

# iceberg_polaris_catalog_role_assignment (Resource)

Assigns a catalog role to a principal role, granting the catalog role's
privileges to all principals in the principal role.

## Example Usage

```terraform
resource "iceberg_polaris_catalog_role_assignment" "assignment" {
  principal_role_name = iceberg_polaris_principal_role.data_engineer.name
  catalog_name        = "my_catalog"
  catalog_role_name   = iceberg_polaris_catalog_role.namespace_admin.name
}
```

## Schema

### Required

- `principal_role_name` (String) The principal role receiving the assignment.
- `catalog_name` (String) The catalog containing the role to assign.
- `catalog_role_name` (String) The catalog role to assign.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
terraform import iceberg_polaris_catalog_role_assignment.example principal_role/catalog/catalog_role
```
