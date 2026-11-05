---
page_title: "iceberg_table Resource - Iceberg"
subcategory: ""
description: |-
  A resource for managing Iceberg tables.
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

# iceberg_table (Resource)

A resource for managing Iceberg tables.

## Example Usage

```terraform
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

resource "iceberg_namespace" "example" {
  name = ["example_namespace"]
  user_properties = {
    description = "An example namespace"
  }
}

resource "iceberg_table" "example" {
  namespace = iceberg_namespace.example.name
  name      = "example_table"

  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      },
      {
        name     = "data"
        type     = "string"
        required = false
      },
      {
        name = "tags"
        type = "list"
        list_properties = {
          element_type     = "string"
          element_required = true
        }
        required = false
      }
    ]
  }
}
```

## Renaming columns

Each field is identified by its `id`, not its name. The provider matches a field
to its existing column by `id` when one is set, and otherwise by name. So renaming
a column without keeping its `id` reads as dropping the old column and adding a new
one — which loses that column's data.

To rename a column while preserving its data, set the field's `id` to the existing
column's id:

```terraform
schema = {
  fields = [
    {
      id       = 1        # keep the id of the column being renamed
      name     = "user_id"
      type     = "long"
      required = true
    }
  ]
}
```

## Referencing source columns by name

Partition and sort fields identify their source columns by field id
(`source_ids` / `source_id`) or by column name (`source_names` /
`source_name`). Set exactly one of each pair, the provider fills in the other.
Because the other attribute is computed, any change to the table shows it as
`(known after apply)` in the plan: `source_names` / `source_name` when you use
ids, and `source_ids` / `source_id` when you use names. The value itself does
not change unless the referenced column does.

Names are resolved against the planned schema at apply time, so a partition or
sort field can name a column added in the same apply until then the plan shows
the ids as `(known after apply)`. Names are case-sensitive, and nested struct
fields use their full dotted name (for example `location.lat`).

```terraform
partition_spec = {
  fields = [
    {
      source_names = ["event_time"]
      name         = "event_time_day"
      transform    = "day"
    }
  ]
}

sort_order = {
  fields = [
    {
      source_name = "location.lat"
      transform   = "identity"
      direction   = "desc"
      null_order  = "nulls-last"
    }
  ]
}
```

A name refers to whichever column has that name when you apply. To rename a
column without changing the partition spec or sort order, keep the column's
`id` and update the name in both places in the same apply.

If columns may be renamed outside Terraform, give the columns that partition
and sort fields reference an explicit `id`. Without one, columns are matched by
name: after an out-of-band rename, the next apply drops the renamed column,
adds an empty column with the configured name, and points the partition spec
and sort order at it.

## Schema

### Required

- `name` (String) The name of the table.
- `namespace` (List of String) The namespace of the table.
- `schema` (Attributes) The schema of the table. (see [below for nested schema](#nestedatt--schema))

### Optional

- `partition_spec` (Attributes) The partition spec of the table. (see [below for nested schema](#nestedatt--partition_spec))
- `sort_order` (Attributes) The sort order of the table. (see [below for nested schema](#nestedatt--sort_order))
- `user_properties` (Map of String) User-defined properties for the table.

### Read-Only

- `id` (String) The ID of this resource.
- `server_properties` (Map of String) Properties returned by the server.

<a id="nestedatt--schema"></a>
### Nested Schema for `schema`

Required:

- `fields` (Attributes List) The fields of the schema (see [below for nested schema](#nestedatt--schema--fields))

Optional:

- `id` (Number) The schema ID.

<a id="nestedatt--schema--fields"></a>
### Nested Schema for `schema.fields`

Required:

- `name` (String) The field name.
- `required` (Boolean) Whether the field is required.
- `type` (String) The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.

Optional:

- `doc` (String) The field documentation.
- `id` (Number) The field ID, stable across updates. Auto-assigned if omitted. To rename a column, keep its id; otherwise it is dropped and re-added.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties))

<a id="nestedatt--schema--fields--list_properties"></a>
### Nested Schema for `schema.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--map_properties"></a>
### Nested Schema for `schema.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Auto-assigned if omitted.
- `value_id` (Number) The map value id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties"></a>
### Nested Schema for `schema.fields.struct_properties`

Required:

- `fields` (Attributes List) The fields of the struct. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields))

<a id="nestedatt--schema--fields--struct_properties--fields"></a>
### Nested Schema for `schema.fields.struct_properties.fields`

Required:

- `name` (String) The field name.
- `required` (Boolean) Whether the field is required.
- `type` (String) The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.

Optional:

- `doc` (String) The field documentation.
- `id` (Number) The field ID, stable across updates. Auto-assigned if omitted. To rename a column, keep its id; otherwise it is dropped and re-added.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Auto-assigned if omitted.
- `value_id` (Number) The map value id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties`

Required:

- `fields` (Attributes List) The fields of the struct. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields`

Required:

- `name` (String) The field name.
- `required` (Boolean) Whether the field is required.
- `type` (String) The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.

Optional:

- `doc` (String) The field documentation.
- `id` (Number) The field ID, stable across updates. Auto-assigned if omitted. To rename a column, keep its id; otherwise it is dropped and re-added.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Auto-assigned if omitted.
- `value_id` (Number) The map value id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties`

Required:

- `fields` (Attributes List) The fields of the struct. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields`

Required:

- `name` (String) The field name.
- `required` (Boolean) Whether the field is required.
- `type` (String) The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.

Optional:

- `doc` (String) The field documentation.
- `id` (Number) The field ID, stable across updates. Auto-assigned if omitted. To rename a column, keep its id; otherwise it is dropped and re-added.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Auto-assigned if omitted.
- `value_id` (Number) The map value id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties`

Required:

- `fields` (Attributes List) The fields of the struct. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields`

Required:

- `name` (String) The field name.
- `required` (Boolean) Whether the field is required.
- `type` (String) The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.

Optional:

- `doc` (String) The field documentation.
- `id` (Number) The field ID, stable across updates. Auto-assigned if omitted. To rename a column, keep its id; otherwise it is dropped and re-added.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Auto-assigned if omitted.
- `value_id` (Number) The map value id. Auto-assigned if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties`

Required:

- `fields` (Attributes List) The fields of the struct. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields`













<a id="nestedatt--partition_spec"></a>
### Nested Schema for `partition_spec`

Required:

- `fields` (Attributes List) The fields of the partition spec. (see [below for nested schema](#nestedatt--partition_spec--fields))

<a id="nestedatt--partition_spec--fields"></a>
### Nested Schema for `partition_spec.fields`

Required:

- `name` (String) The partition field name.
- `transform` (String) The partition transform.

Optional:

- `field_id` (Number) The partition field ID.
- `source_ids` (List of Number) The source field IDs. Set exactly one of source_ids or source_names the other is computed.
- `source_names` (List of String) The source column names, resolved to source_ids at apply time. Nested struct fields use their full dotted name (e.g. 'location.lat'). Set exactly one of source_ids or source_names the other is computed.



<a id="nestedatt--sort_order"></a>
### Nested Schema for `sort_order`

Required:

- `fields` (Attributes List) The fields of the sort order. (see [below for nested schema](#nestedatt--sort_order--fields))

<a id="nestedatt--sort_order--fields"></a>
### Nested Schema for `sort_order.fields`

Required:

- `direction` (String) The sort direction (asc or desc).
- `null_order` (String) The null order (nulls-first or nulls-last).
- `transform` (String) The sort transform.

Optional:

- `source_id` (Number) The source field ID. Set exactly one of source_id or source_name the other is computed.
- `source_name` (String) The source column name, resolved to source_id at apply time. Nested struct fields use their full dotted name (e.g. 'location.lat'). Set exactly one of source_id or source_name the other is computed.

## Import

Import is supported using the following syntax:

```shell
$ terraform import iceberg_table a.b.table_name
```

Configuration generated with `terraform plan -generate-config-out` sets both
`source_ids` and `source_names` on each partition field, and both `source_id`
and `source_name` on each sort field. Keep one attribute of each pair and
remove the other.
