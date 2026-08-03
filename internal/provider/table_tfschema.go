// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rscschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const tableSchemaFieldMaxDepth = 4

func tableSchemaResourceAttributes() map[string]rscschema.Attribute {
	return map[string]rscschema.Attribute{
		"id": rscschema.Int64Attribute{
			Description: "The schema ID.",
			Optional:    true,
			Computed:    true,
		},
		"fields": rscschema.ListNestedAttribute{
			Description: "The fields of the schema",
			Required:    true,
			NestedObject: rscschema.NestedAttributeObject{
				Attributes: tableSchemaFieldResourceAttributes(tableSchemaFieldMaxDepth),
			},
		},
	}
}

func tableSchemaDataSourceAttributes() map[string]dschema.Attribute {
	return map[string]dschema.Attribute{
		"id": dschema.Int64Attribute{
			Description: "The schema ID.",
			Computed:    true,
		},
		"fields": dschema.ListNestedAttribute{
			Description: "The fields of the schema.",
			Computed:    true,
			NestedObject: dschema.NestedAttributeObject{
				Attributes: tableSchemaFieldDataSourceAttributes(tableSchemaFieldMaxDepth),
			},
		},
	}
}

func tablePartitionSpecResourceAttributes() map[string]rscschema.Attribute {
	return map[string]rscschema.Attribute{
		"spec_id": rscschema.Int64Attribute{
			Description: "The partition spec ID.",
			Computed:    true,
		},
		"fields": rscschema.ListNestedAttribute{
			Description: "The fields of the partition spec.",
			Required:    true,
			NestedObject: rscschema.NestedAttributeObject{
				Attributes: tablePartitionSpecFieldResourceAttributes(),
			},
		},
	}
}

func tablePartitionSpecDataSourceAttributes() map[string]dschema.Attribute {
	return map[string]dschema.Attribute{
		"spec_id": dschema.Int64Attribute{
			Description: "The partition spec ID.",
			Computed:    true,
		},
		"fields": dschema.ListNestedAttribute{
			Description: "The fields of the partition spec.",
			Computed:    true,
			NestedObject: dschema.NestedAttributeObject{
				Attributes: tablePartitionSpecFieldDataSourceAttributes(),
			},
		},
	}
}

func tableSortOrderResourceAttributes() map[string]rscschema.Attribute {
	return map[string]rscschema.Attribute{
		"order_id": rscschema.Int64Attribute{
			Description: "The sort order ID.",
			Computed:    true,
		},
		"fields": rscschema.ListNestedAttribute{
			Description: "The fields of the sort order.",
			Required:    true,
			NestedObject: rscschema.NestedAttributeObject{
				Attributes: tableSortOrderFieldResourceAttributes(),
			},
		},
	}
}

func tableSortOrderDataSourceAttributes() map[string]dschema.Attribute {
	return map[string]dschema.Attribute{
		"order_id": dschema.Int64Attribute{
			Description: "The sort order ID.",
			Computed:    true,
		},
		"fields": dschema.ListNestedAttribute{
			Description: "The fields of the sort order.",
			Computed:    true,
			NestedObject: dschema.NestedAttributeObject{
				Attributes: tableSortOrderFieldDataSourceAttributes(),
			},
		},
	}
}

func tableSchemaFieldResourceAttributes(depth int) map[string]rscschema.Attribute {
	attrs := map[string]rscschema.Attribute{
		"id": rscschema.Int64Attribute{
			Description: "The field ID, stable across updates. Auto-assigned if omitted. To rename a column, keep its id; otherwise it is dropped and re-added.",
			Optional:    true,
			Computed:    true,
		},
		"name": rscschema.StringAttribute{
			Description: "The field name.",
			Required:    true,
		},
		"type": rscschema.StringAttribute{
			Description: "The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.",
			Required:    true,
		},
		"required": rscschema.BoolAttribute{
			Description: "Whether the field is required.",
			Required:    true,
		},
		"doc": rscschema.StringAttribute{
			Description: "The field documentation.",
			Optional:    true,
		},
		"list_properties": rscschema.SingleNestedAttribute{
			Description: "Properties for list type.",
			Optional:    true,
			Attributes: map[string]rscschema.Attribute{
				"element_id": rscschema.Int64Attribute{
					Description: "The list element id. Auto-assigned if omitted.",
					Optional:    true,
					Computed:    true,
				},
				"element_type": rscschema.StringAttribute{
					Description: "The list element type.",
					Required:    true,
				},
				"element_required": rscschema.BoolAttribute{
					Description: "Whether the list element is required.",
					Required:    true,
				},
			},
		},
		"map_properties": rscschema.SingleNestedAttribute{
			Description: "Properties for map type.",
			Optional:    true,
			Attributes: map[string]rscschema.Attribute{
				"key_id": rscschema.Int64Attribute{
					Description: "The map key id. Auto-assigned if omitted.",
					Optional:    true,
					Computed:    true,
				},
				"key_type": rscschema.StringAttribute{
					Description: "The map key type.",
					Required:    true,
				},
				"value_id": rscschema.Int64Attribute{
					Description: "The map value id. Auto-assigned if omitted.",
					Optional:    true,
					Computed:    true,
				},
				"value_type": rscschema.StringAttribute{
					Description: "The map value type.",
					Required:    true,
				},
				"value_required": rscschema.BoolAttribute{
					Description: "Whether the map value is required.",
					Required:    true,
				},
			},
		},
	}

	if depth > 0 {
		attrs["struct_properties"] = rscschema.SingleNestedAttribute{
			Description: "Properties for struct type.",
			Optional:    true,
			Attributes: map[string]rscschema.Attribute{
				"fields": rscschema.ListNestedAttribute{
					Description: "The fields of the struct.",
					Required:    true,
					NestedObject: rscschema.NestedAttributeObject{
						Attributes: tableSchemaFieldResourceAttributes(depth - 1),
					},
				},
			},
		}
	} else {
		attrs["struct_properties"] = rscschema.SingleNestedAttribute{
			Description: "Properties for struct type.",
			Optional:    true,
			Attributes: map[string]rscschema.Attribute{
				"fields": rscschema.ListNestedAttribute{
					Description: "The fields of the struct.",
					Required:    true,
					NestedObject: rscschema.NestedAttributeObject{
						Attributes: map[string]rscschema.Attribute{},
					},
				},
			},
		}
	}

	return attrs
}

func tableSchemaFieldDataSourceAttributes(depth int) map[string]dschema.Attribute {
	attrs := map[string]dschema.Attribute{
		"id": dschema.Int64Attribute{
			Description: "The field ID.",
			Computed:    true,
		},
		"name": dschema.StringAttribute{
			Description: "The field name.",
			Computed:    true,
		},
		"type": dschema.StringAttribute{
			Description: "The field type (e.g., 'int', 'string', 'decimal(10,2)', 'struct'). For struct, use struct_properties.",
			Computed:    true,
		},
		"required": dschema.BoolAttribute{
			Description: "Whether the field is required.",
			Computed:    true,
		},
		"doc": dschema.StringAttribute{
			Description: "The field documentation.",
			Computed:    true,
		},
		"list_properties": dschema.SingleNestedAttribute{
			Description: "Properties for list type.",
			Computed:    true,
			Attributes: map[string]dschema.Attribute{
				"element_id": dschema.Int64Attribute{
					Description: "The list element id.",
					Computed:    true,
				},
				"element_type": dschema.StringAttribute{
					Description: "The list element type.",
					Computed:    true,
				},
				"element_required": dschema.BoolAttribute{
					Description: "Whether the list element is required.",
					Computed:    true,
				},
			},
		},
		"map_properties": dschema.SingleNestedAttribute{
			Description: "Properties for map type.",
			Computed:    true,
			Attributes: map[string]dschema.Attribute{
				"key_id": dschema.Int64Attribute{
					Description: "The map key id.",
					Computed:    true,
				},
				"key_type": dschema.StringAttribute{
					Description: "The map key type.",
					Computed:    true,
				},
				"value_id": dschema.Int64Attribute{
					Description: "The map value id.",
					Computed:    true,
				},
				"value_type": dschema.StringAttribute{
					Description: "The map value type.",
					Computed:    true,
				},
				"value_required": dschema.BoolAttribute{
					Description: "Whether the map value is required.",
					Computed:    true,
				},
			},
		},
	}

	if depth > 0 {
		attrs["struct_properties"] = dschema.SingleNestedAttribute{
			Description: "Properties for struct type.",
			Computed:    true,
			Attributes: map[string]dschema.Attribute{
				"fields": dschema.ListNestedAttribute{
					Description: "The fields of the struct.",
					Computed:    true,
					NestedObject: dschema.NestedAttributeObject{
						Attributes: tableSchemaFieldDataSourceAttributes(depth - 1),
					},
				},
			},
		}
	} else {
		attrs["struct_properties"] = dschema.SingleNestedAttribute{
			Description: "Properties for struct type.",
			Computed:    true,
			Attributes: map[string]dschema.Attribute{
				"fields": dschema.ListNestedAttribute{
					Description: "The fields of the struct.",
					Computed:    true,
					NestedObject: dschema.NestedAttributeObject{
						Attributes: map[string]dschema.Attribute{},
					},
				},
			},
		}
	}

	return attrs
}

func tablePartitionSpecFieldResourceAttributes() map[string]rscschema.Attribute {
	return map[string]rscschema.Attribute{
		"source_ids": rscschema.ListAttribute{
			Description: "The source field IDs.",
			Required:    true,
			ElementType: types.Int64Type,
		},
		"field_id": rscschema.Int64Attribute{
			Description: "The partition field ID.",
			Optional:    true,
			Computed:    true,
		},
		"name": rscschema.StringAttribute{
			Description: "The partition field name.",
			Required:    true,
		},
		"transform": rscschema.StringAttribute{
			Description: "The partition transform.",
			Required:    true,
		},
	}
}

func tablePartitionSpecFieldDataSourceAttributes() map[string]dschema.Attribute {
	return map[string]dschema.Attribute{
		"source_ids": dschema.ListAttribute{
			Description: "The source field IDs.",
			Computed:    true,
			ElementType: types.Int64Type,
		},
		"field_id": dschema.Int64Attribute{
			Description: "The partition field ID.",
			Computed:    true,
		},
		"name": dschema.StringAttribute{
			Description: "The partition field name.",
			Computed:    true,
		},
		"transform": dschema.StringAttribute{
			Description: "The partition transform.",
			Computed:    true,
		},
	}
}

func tableSortOrderFieldResourceAttributes() map[string]rscschema.Attribute {
	return map[string]rscschema.Attribute{
		"source_id": rscschema.Int64Attribute{
			Description: "The source field ID.",
			Required:    true,
		},
		"transform": rscschema.StringAttribute{
			Description: "The sort transform.",
			Required:    true,
		},
		"direction": rscschema.StringAttribute{
			Description: "The sort direction (asc or desc).",
			Required:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("asc", "desc"),
			},
		},
		"null_order": rscschema.StringAttribute{
			Description: "The null order (nulls-first or nulls-last).",
			Required:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("nulls-first", "nulls-last"),
			},
		},
	}
}

func tableSortOrderFieldDataSourceAttributes() map[string]dschema.Attribute {
	return map[string]dschema.Attribute{
		"source_id": dschema.Int64Attribute{
			Description: "The source field ID.",
			Computed:    true,
		},
		"transform": dschema.StringAttribute{
			Description: "The sort transform.",
			Computed:    true,
		},
		"direction": dschema.StringAttribute{
			Description: "The sort direction (asc or desc).",
			Computed:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("asc", "desc"),
			},
		},
		"null_order": dschema.StringAttribute{
			Description: "The null order (nulls-first or nulls-last).",
			Computed:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("nulls-first", "nulls-last"),
			},
		},
	}
}
