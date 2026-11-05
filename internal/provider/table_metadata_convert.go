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
	"context"

	"github.com/apache/iceberg-go/table"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// icebergTableMetadataFields holds Terraform values converted from an Iceberg table's metadata
// (schema, partition spec, sort order, properties). Used by the table resource and table data source.
type icebergTableMetadataFields struct {
	Schema           types.Object
	PartitionSpec    types.Object
	SortOrder        types.Object
	ServerProperties types.Map
}

func convertIcebergTableMetadataToModel(ctx context.Context, tbl *table.Table, out *icebergTableMetadataFields, diags *diag.Diagnostics) {
	serverProperties, d := types.MapValueFrom(ctx, types.StringType, tbl.Properties())
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	out.ServerProperties = serverProperties

	icebergSchema := tbl.Schema()
	var updatedSchema icebergTableSchema
	if err := updatedSchema.FromIceberg(icebergSchema); err != nil {
		diags.AddError("failed to convert iceberg schema to terraform schema", err.Error())

		return
	}
	var d2 diag.Diagnostics
	out.Schema, d2 = types.ObjectValueFrom(ctx, icebergTableSchema{}.AttrTypes(), updatedSchema)
	diags.Append(d2...)
	if diags.HasError() {
		return
	}

	icebergSpec := tbl.Spec()
	if icebergSpec.NumFields() > 0 {
		var updatedSpec icebergTablePartitionSpec
		if err := updatedSpec.FromIceberg(icebergSpec, icebergSchema); err != nil {
			diags.AddError("failed to convert iceberg partition spec to terraform partition spec", err.Error())

			return
		}
		var d3 diag.Diagnostics
		out.PartitionSpec, d3 = types.ObjectValueFrom(ctx, icebergTablePartitionSpec{}.AttrTypes(), updatedSpec)
		diags.Append(d3...)
	} else {
		out.PartitionSpec = types.ObjectNull(icebergTablePartitionSpec{}.AttrTypes())
	}

	icebergOrder := tbl.SortOrder()
	if icebergOrder.Len() > 0 {
		var updatedOrder icebergTableSortOrder
		if err := updatedOrder.FromIceberg(icebergOrder, icebergSchema); err != nil {
			diags.AddError("failed to convert iceberg sort order to terraform sort order", err.Error())

			return
		}
		var d4 diag.Diagnostics
		out.SortOrder, d4 = types.ObjectValueFrom(ctx, icebergTableSortOrder{}.AttrTypes(), updatedOrder)
		diags.Append(d4...)
	} else {
		out.SortOrder = types.ObjectNull(icebergTableSortOrder{}.AttrTypes())
	}
}
