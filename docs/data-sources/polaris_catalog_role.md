---
page_title: "iceberg_polaris_catalog_role Data Source - Iceberg"
subcategory: ""
description: |-
  Look up a Polaris catalog role by name and catalog.
---

# iceberg_polaris_catalog_role (Data Source)

Look up a Polaris catalog role by name and catalog.

## Example Usage

```terraform
data "iceberg_polaris_catalog_role" "existing" {
  catalog_name = "my_catalog"
  name         = "table_reader"
}
```

## Schema

### Required

- `catalog_name` (String) The catalog containing the role.
- `name` (String) The name of the catalog role.

### Read-Only

- `id` (String) The ID of this resource.
- `properties` (Map of String) Arbitrary metadata properties for the catalog role.
