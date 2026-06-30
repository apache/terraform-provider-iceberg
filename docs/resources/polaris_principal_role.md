---
page_title: "iceberg_polaris_principal_role Resource - Iceberg"
subcategory: ""
description: |-
  A resource for managing Polaris principal roles.
---

# iceberg_polaris_principal_role (Resource)

A resource for managing Polaris principal roles.

## Example Usage

```terraform
resource "iceberg_polaris_principal_role" "data_engineer" {
  name = "data_engineer"
  properties = {
    department = "engineering"
  }
}
```

## Schema

### Required

- `name` (String) The name of the principal role.

### Optional

- `federated` (Bool) Whether the role is managed by an external identity provider. Immutable after creation.
- `properties` (Map of String) Arbitrary metadata properties for the principal role.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
terraform import iceberg_polaris_principal_role.example my_role_name
```
