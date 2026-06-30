---
page_title: "iceberg_polaris_grant Resource - Iceberg"
subcategory: ""
description: |-
  Grants a privilege on a securable object to a catalog role.
---

# iceberg_polaris_grant (Resource)

Grants a privilege on a securable object to a catalog role.

## Example Usage

```terraform
# Grant on a namespace
resource "iceberg_polaris_grant" "namespace_read" {
  catalog_name      = "my_catalog"
  catalog_role_name = iceberg_polaris_catalog_role.namespace_admin.name
  privilege         = "NAMESPACE_LIST"
  namespace         = ["my_namespace"]
}

# Grant on the entire catalog
resource "iceberg_polaris_grant" "catalog_admin" {
  catalog_name      = "my_catalog"
  catalog_role_name = iceberg_polaris_catalog_role.admin.name
  privilege         = "CATALOG_MANAGE_CONTENT"
  catalog           = "my_catalog"
}
```

## Schema

### Required

- `catalog_name` (String) The catalog containing the role.
- `catalog_role_name` (String) The catalog role receiving the grant.
- `privilege` (String) The privilege to grant.

### Optional

- `catalog` (String) Grant on the whole catalog. Mutually exclusive with namespace/table/view.
- `namespace` (List of String) Namespace to grant on. Required if table or view is set.
- `table` (String) Table name to grant on. Requires namespace.
- `view` (String) View name to grant on. Requires namespace.
- `cascade` (Bool) Cascade revocation to subresources (only used on destroy).

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
terraform import iceberg_polaris_grant.example catalog_name/role_name/CATALOG/null/CATALOG_MANAGE_CONTENT
terraform import iceberg_polaris_grant.example catalog_name/role_name/NAMESPACE/my_namespace/NAMESPACE_LIST
terraform import iceberg_polaris_grant.example catalog_name/role_name/TABLE/my_namespace.my_table/TABLE_READ_DATA
```
