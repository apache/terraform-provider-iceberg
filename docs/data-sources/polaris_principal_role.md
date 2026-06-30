---
page_title: "iceberg_polaris_principal_role Data Source - Iceberg"
subcategory: ""
description: |-
  Look up a Polaris principal role by name.
---

# iceberg_polaris_principal_role (Data Source)

Look up a Polaris principal role by name.

## Example Usage

```terraform
data "iceberg_polaris_principal_role" "existing" {
  name = "admin"
}
```

## Schema

### Required

- `name` (String) The name of the principal role.

### Read-Only

- `id` (String) The ID of this resource.
- `federated` (Bool) Whether the role is managed by an external identity provider.
- `properties` (Map of String) Arbitrary metadata properties for the principal role.
