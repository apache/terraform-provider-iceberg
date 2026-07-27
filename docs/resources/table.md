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
- `id` (Number) The field ID. Assigned automatically by the provider if omitted.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties))

<a id="nestedatt--schema--fields--list_properties"></a>
### Nested Schema for `schema.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Assigned automatically by the provider if omitted.


<a id="nestedatt--schema--fields--map_properties"></a>
### Nested Schema for `schema.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Assigned automatically by the provider if omitted.
- `value_id` (Number) The map value id. Assigned automatically by the provider if omitted.


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
- `id` (Number) The field ID. Assigned automatically by the provider if omitted.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Assigned automatically by the provider if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Assigned automatically by the provider if omitted.
- `value_id` (Number) The map value id. Assigned automatically by the provider if omitted.


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
- `id` (Number) The field ID. Assigned automatically by the provider if omitted.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Assigned automatically by the provider if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Assigned automatically by the provider if omitted.
- `value_id` (Number) The map value id. Assigned automatically by the provider if omitted.


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
- `id` (Number) The field ID. Assigned automatically by the provider if omitted.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Assigned automatically by the provider if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Assigned automatically by the provider if omitted.
- `value_id` (Number) The map value id. Assigned automatically by the provider if omitted.


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
- `id` (Number) The field ID. Assigned automatically by the provider if omitted.
- `list_properties` (Attributes) Properties for list type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties))
- `map_properties` (Attributes) Properties for map type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties))
- `struct_properties` (Attributes) Properties for struct type. (see [below for nested schema](#nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties))

<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--list_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.list_properties`

Required:

- `element_required` (Boolean) Whether the list element is required.
- `element_type` (String) The list element type.

Optional:

- `element_id` (Number) The list element id. Assigned automatically by the provider if omitted.


<a id="nestedatt--schema--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--struct_properties--fields--map_properties"></a>
### Nested Schema for `schema.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.struct_properties.fields.map_properties`

Required:

- `key_type` (String) The map key type.
- `value_required` (Boolean) Whether the map value is required.
- `value_type` (String) The map value type.

Optional:

- `key_id` (Number) The map key id. Assigned automatically by the provider if omitted.
- `value_id` (Number) The map value id. Assigned automatically by the provider if omitted.


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
- `source_ids` (List of Number) The source field IDs.
- `transform` (String) The partition transform.

Optional:

- `field_id` (Number) The partition field ID.



<a id="nestedatt--sort_order"></a>
### Nested Schema for `sort_order`

Required:

- `fields` (Attributes List) The fields of the sort order. (see [below for nested schema](#nestedatt--sort_order--fields))

<a id="nestedatt--sort_order--fields"></a>
### Nested Schema for `sort_order.fields`

Required:

- `direction` (String) The sort direction (asc or desc).
- `null_order` (String) The null order (nulls-first or nulls-last).
- `source_id` (Number) The source field ID.
- `transform` (String) The sort transform.

## Import

Import is supported using the following syntax:

```shell
$ terraform import iceberg_table a.b.table_name
