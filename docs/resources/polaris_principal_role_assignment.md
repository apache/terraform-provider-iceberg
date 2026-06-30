---
page_title: "iceberg_polaris_principal_role_assignment Resource - Iceberg"
subcategory: ""
description: |-
  Assigns a principal role to a Polaris principal.
---

# iceberg_polaris_principal_role_assignment (Resource)

Assigns a principal role to a Polaris principal.

## Example Usage

```terraform
resource "iceberg_polaris_principal_role_assignment" "assignment" {
  principal_name      = iceberg_polaris_principal.my_principal.name
  principal_role_name = iceberg_polaris_principal_role.data_engineer.name
}
```

## Schema

### Required

- `principal_name` (String) The name of the principal to assign the role to.
- `principal_role_name` (String) The name of the principal role to assign.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
terraform import iceberg_polaris_principal_role_assignment.example principal_name/role_name
```
