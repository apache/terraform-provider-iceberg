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
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// schemaOf builds a single-field schema for "data" with id 2, alongside a
// fixed required "id" field so the fixtures resemble a real table.
func schemaOf(dataType iceberg.Type, dataRequired bool) *iceberg.Schema {
	return iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: dataType, Required: dataRequired},
	)
}

func listOf(element iceberg.Type) iceberg.Type {
	return &iceberg.ListType{ElementID: 3, Element: element, ElementRequired: false}
}

func structOf(fieldType iceberg.Type) iceberg.Type {
	return &iceberg.StructType{FieldList: []iceberg.NestedField{
		{ID: 3, Name: "nested", Type: fieldType, Required: false},
	}}
}

func TestValidateSchemaEvolution(t *testing.T) {
	tests := []struct {
		name    string
		current *iceberg.Schema
		planned *iceberg.Schema
		wantErr string
	}{
		{
			name:    "unchanged",
			current: schemaOf(iceberg.PrimitiveTypes.String, false),
			planned: schemaOf(iceberg.PrimitiveTypes.String, false),
		},
		{
			name:    "rename",
			current: iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "old", Type: iceberg.PrimitiveTypes.String}),
			planned: iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "new", Type: iceberg.PrimitiveTypes.String}),
		},
		{
			name:    "add field",
			current: iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true}),
			planned: schemaOf(iceberg.PrimitiveTypes.String, false),
		},
		{
			name:    "drop field",
			current: schemaOf(iceberg.PrimitiveTypes.String, false),
			planned: iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true}),
		},
		{
			name:    "int to long",
			current: schemaOf(iceberg.PrimitiveTypes.Int32, false),
			planned: schemaOf(iceberg.PrimitiveTypes.Int64, false),
		},
		{
			name:    "float to double",
			current: schemaOf(iceberg.PrimitiveTypes.Float32, false),
			planned: schemaOf(iceberg.PrimitiveTypes.Float64, false),
		},
		{
			name:    "decimal widened precision",
			current: schemaOf(iceberg.DecimalTypeOf(9, 2), false),
			planned: schemaOf(iceberg.DecimalTypeOf(18, 2), false),
		},
		{
			name:    "required to optional",
			current: schemaOf(iceberg.PrimitiveTypes.String, true),
			planned: schemaOf(iceberg.PrimitiveTypes.String, false),
		},
		{
			name:    "list element promoted",
			current: schemaOf(listOf(iceberg.PrimitiveTypes.Int32), false),
			planned: schemaOf(listOf(iceberg.PrimitiveTypes.Int64), false),
		},
		{
			name:    "long to string",
			current: schemaOf(iceberg.PrimitiveTypes.Int64, false),
			planned: schemaOf(iceberg.PrimitiveTypes.String, false),
			wantErr: `field "data" (id 2): string is not a valid type promotion from long`,
		},
		{
			name:    "string to long",
			current: schemaOf(iceberg.PrimitiveTypes.String, false),
			planned: schemaOf(iceberg.PrimitiveTypes.Int64, false),
			wantErr: `field "data" (id 2): long is not a valid type promotion from string`,
		},
		{
			name:    "long to int",
			current: schemaOf(iceberg.PrimitiveTypes.Int64, false),
			planned: schemaOf(iceberg.PrimitiveTypes.Int32, false),
			wantErr: `field "data" (id 2): int is not a valid type promotion from long`,
		},
		{
			name:    "double to float",
			current: schemaOf(iceberg.PrimitiveTypes.Float64, false),
			planned: schemaOf(iceberg.PrimitiveTypes.Float32, false),
			wantErr: `field "data" (id 2): float is not a valid type promotion from double`,
		},
		{
			name:    "decimal reduced precision",
			current: schemaOf(iceberg.DecimalTypeOf(18, 2), false),
			planned: schemaOf(iceberg.DecimalTypeOf(9, 2), false),
			wantErr: `field "data" (id 2): decimal(9, 2) is not a valid type promotion from decimal(18, 2)`,
		},
		{
			name:    "optional to required",
			current: schemaOf(iceberg.PrimitiveTypes.String, false),
			planned: schemaOf(iceberg.PrimitiveTypes.String, true),
			wantErr: `field "data" (id 2): changing a field from optional to required is not an allowed schema evolution`,
		},
		{
			name:    "primitive to struct",
			current: schemaOf(iceberg.PrimitiveTypes.String, false),
			planned: schemaOf(structOf(iceberg.PrimitiveTypes.String), false),
			wantErr: `field "data" (id 2): cannot change field type from string to struct<`,
		},
		{
			name:    "struct to primitive",
			current: schemaOf(structOf(iceberg.PrimitiveTypes.String), false),
			planned: schemaOf(iceberg.PrimitiveTypes.String, false),
			wantErr: `field "data" (id 2): cannot change field type from struct<`,
		},
		{
			name:    "list to struct",
			current: schemaOf(listOf(iceberg.PrimitiveTypes.String), false),
			planned: schemaOf(structOf(iceberg.PrimitiveTypes.String), false),
			wantErr: `field "data" (id 2): cannot change field type from list<string> to struct<`,
		},
		{
			name:    "list element demoted",
			current: schemaOf(listOf(iceberg.PrimitiveTypes.Int64), false),
			planned: schemaOf(listOf(iceberg.PrimitiveTypes.String), false),
			wantErr: `field "element" (id 3): string is not a valid type promotion from long`,
		},
		{
			name:    "struct field demoted",
			current: schemaOf(structOf(iceberg.PrimitiveTypes.Int64), false),
			planned: schemaOf(structOf(iceberg.PrimitiveTypes.String), false),
			wantErr: `field "nested" (id 3): string is not a valid type promotion from long`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchemaEvolution(tt.current, tt.planned)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateFieldIDsReservedRange(t *testing.T) {
	schemaWithIDs := func(ids ...int64) icebergTableSchema {
		var s icebergTableSchema
		for i, id := range ids {
			s.Fields = append(s.Fields, icebergTableSchemaField{
				ID:   types.Int64Value(id),
				Name: fmt.Sprintf("f%d", i),
				Type: "long",
			})
		}

		return s
	}

	tests := []struct {
		name    string
		schema  icebergTableSchema
		wantErr string
	}{
		{name: "max usable id", schema: schemaWithIDs(1, 2147483447)},
		{name: "first reserved id", schema: schemaWithIDs(1, 2147483448), wantErr: "id 2147483448"},
		{name: "last reserved id", schema: schemaWithIDs(1, 2147483647), wantErr: "id 2147483647"},
		{name: "negative id", schema: schemaWithIDs(1, -1), wantErr: "id -1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.validateFieldIDs()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAssignFieldIDsIntoReservedRange(t *testing.T) {
	// The fresh id assigned after 2147483447 is reserved.
	s := icebergTableSchema{Fields: []icebergTableSchemaField{
		{ID: types.Int64Value(2147483447), Name: "a", Type: "long"},
		{ID: types.Int64Null(), Name: "b", Type: "long"},
	}}
	s.assignFieldIDs(0)
	if err := s.validateFieldIDs(); err == nil || !strings.Contains(err.Error(), "id 2147483448") {
		t.Fatalf("expected reserved id error, got %v", err)
	}
}

func TestPartitionSpecSourceIDsRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		specJSON  string
		sourceIDs []int64
	}{
		{
			name:      "single source",
			specJSON:  `{"spec-id": 1, "fields": [{"source-id": 1, "field-id": 1000, "name": "id_bucket", "transform": "bucket[16]"}]}`,
			sourceIDs: []int64{1},
		},
		{
			name:      "multi source",
			specJSON:  `{"spec-id": 1, "fields": [{"source-ids": [1, 2], "field-id": 1000, "name": "id_data_bucket", "transform": "bucket[16]"}]}`,
			sourceIDs: []int64{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want iceberg.PartitionSpec
			if err := json.Unmarshal([]byte(tt.specJSON), &want); err != nil {
				t.Fatalf("unmarshal spec: %v", err)
			}

			var s icebergTablePartitionSpec
			if err := s.FromIceberg(want, schemaOf(iceberg.PrimitiveTypes.String, false)); err != nil {
				t.Fatalf("FromIceberg: %v", err)
			}
			if got := int64sOf(t, s.Fields[0].SourceIDs); !slices.Equal(got, tt.sourceIDs) {
				t.Errorf("expected source ids %v, got %v", tt.sourceIDs, got)
			}

			got, err := s.ToIceberg(schemaOf(iceberg.PrimitiveTypes.String, false))
			if err != nil {
				t.Fatalf("ToIceberg: %v", err)
			}
			if !got.Equals(want) {
				t.Errorf("expected spec %v after round trip, got %v", want, *got)
			}
		})
	}
}

func int64List(ids ...int64) types.List {
	elems := make([]attr.Value, len(ids))
	for i, id := range ids {
		elems[i] = types.Int64Value(id)
	}

	return types.ListValueMust(types.Int64Type, elems)
}

func stringList(names ...string) types.List {
	elems := make([]attr.Value, len(names))
	for i, name := range names {
		elems[i] = types.StringValue(name)
	}

	return types.ListValueMust(types.StringType, elems)
}

func int64sOf(t *testing.T, l types.List) []int64 {
	t.Helper()
	if l.IsNull() || l.IsUnknown() {
		t.Fatalf("expected a known list, got %v", l)
	}
	elems := l.Elements()
	out := make([]int64, len(elems))
	for i, e := range elems {
		out[i] = e.(types.Int64).ValueInt64()
	}

	return out
}

func stringsOf(t *testing.T, l types.List) []string {
	t.Helper()
	if l.IsNull() || l.IsUnknown() {
		t.Fatalf("expected a known list, got %v", l)
	}
	elems := l.Elements()
	out := make([]string, len(elems))
	for i, e := range elems {
		out[i] = e.(types.String).ValueString()
	}

	return out
}

// sourceRefSchema has a top-level column, a struct column, and a list of structs
// whose element field also answers to the short alias "points.x".
func sourceRefSchema() *iceberg.Schema {
	return iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "event_time", Type: iceberg.PrimitiveTypes.TimestampTz},
		iceberg.NestedField{ID: 3, Name: "location", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
			{ID: 5, Name: "lat", Type: iceberg.PrimitiveTypes.Float64},
		}}},
		iceberg.NestedField{ID: 4, Name: "points", Type: &iceberg.ListType{
			ElementID: 6,
			Element: &iceberg.StructType{FieldList: []iceberg.NestedField{
				{ID: 7, Name: "x", Type: iceberg.PrimitiveTypes.Float64},
			}},
		}},
	)
}

func partitionFieldByName(names ...string) icebergTablePartitionField {
	return icebergTablePartitionField{
		SourceIDs:   types.ListNull(types.Int64Type),
		SourceNames: stringList(names...),
		FieldID:     types.Int64Unknown(),
		Name:        "p",
		Transform:   "identity",
	}
}

func sortFieldByName(name string) icebergTableSortField {
	return icebergTableSortField{
		SourceID:   types.Int64Null(),
		SourceName: types.StringValue(name),
		Transform:  "identity",
		Direction:  "asc",
		NullOrder:  "nulls-first",
	}
}

func TestPartitionSpecResolveSourceIDs(t *testing.T) {
	unknownIDs := partitionFieldByName("id")
	unknownIDs.SourceIDs = types.ListUnknown(types.Int64Type)

	idsSet := partitionFieldByName("id")
	idsSet.SourceIDs = int64List(9)

	idsSetNamesUnknown := partitionFieldByName()
	idsSetNamesUnknown.SourceIDs = int64List(1)
	idsSetNamesUnknown.SourceNames = types.ListUnknown(types.StringType)

	tests := []struct {
		name    string
		field   icebergTablePartitionField
		want    []int64
		wantErr string
	}{
		{name: "top-level column", field: partitionFieldByName("event_time"), want: []int64{2}},
		{name: "nested struct field", field: partitionFieldByName("location.lat"), want: []int64{5}},
		{name: "list element struct field", field: partitionFieldByName("points.element.x"), want: []int64{7}},
		{name: "multiple sources keep order", field: partitionFieldByName("event_time", "id"), want: []int64{2, 1}},
		{name: "unknown ids", field: unknownIDs, want: []int64{1}},
		{name: "ids already set", field: idsSet, want: []int64{9}},
		{name: "ids set and names unknown", field: idsSetNamesUnknown, want: []int64{1}},
		{name: "unknown column", field: partitionFieldByName("missing"), wantErr: `partition field "p": source column "missing" not found`},
		{name: "case mismatch", field: partitionFieldByName("Event_Time"), wantErr: `source column "Event_Time" not found`},
		{name: "short alias", field: partitionFieldByName("points.x"), wantErr: `source column "points.x" must be referenced by its full name "points.element.x"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := icebergTablePartitionSpec{Fields: []icebergTablePartitionField{tt.field}}
			err := spec.resolveSourceIDs(sourceRefSchema())

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}

				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got := int64sOf(t, spec.Fields[0].SourceIDs); !slices.Equal(got, tt.want) {
				t.Errorf("expected source ids %v, got %v", tt.want, got)
			}
		})
	}
}

func TestSortOrderResolveSourceIDs(t *testing.T) {
	unknownID := sortFieldByName("id")
	unknownID.SourceID = types.Int64Unknown()

	idSet := sortFieldByName("id")
	idSet.SourceID = types.Int64Value(9)

	idSetNameUnknown := sortFieldByName("")
	idSetNameUnknown.SourceID = types.Int64Value(1)
	idSetNameUnknown.SourceName = types.StringUnknown()

	tests := []struct {
		name    string
		field   icebergTableSortField
		want    int64
		wantErr string
	}{
		{name: "top-level column", field: sortFieldByName("event_time"), want: 2},
		{name: "nested struct field", field: sortFieldByName("location.lat"), want: 5},
		{name: "list element struct field", field: sortFieldByName("points.element.x"), want: 7},
		{name: "unknown id", field: unknownID, want: 1},
		{name: "id already set", field: idSet, want: 9},
		{name: "id set and name unknown", field: idSetNameUnknown, want: 1},
		{name: "unknown column", field: sortFieldByName("missing"), wantErr: `sort_order.fields[0]: source column "missing" not found`},
		{name: "case mismatch", field: sortFieldByName("ID"), wantErr: `source column "ID" not found`},
		{name: "short alias", field: sortFieldByName("points.x"), wantErr: `source column "points.x" must be referenced by its full name "points.element.x"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := icebergTableSortOrder{Fields: []icebergTableSortField{tt.field}}
			err := order.resolveSourceIDs(sourceRefSchema())

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}

				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got := order.Fields[0].SourceID.ValueInt64(); got != tt.want {
				t.Errorf("expected source id %d, got %d", tt.want, got)
			}
		})
	}
}

func TestPartitionSpecFromIcebergSourceNames(t *testing.T) {
	var spec iceberg.PartitionSpec
	specJSON := `{"spec-id": 1, "fields": [
		{"source-id": 2, "field-id": 1000, "name": "event_time_day", "transform": "day"},
		{"source-id": 5, "field-id": 1001, "name": "lat", "transform": "identity"},
		{"source-ids": [1, 2], "field-id": 1002, "name": "multi", "transform": "bucket[16]"},
		{"source-ids": [1, 99], "field-id": 1003, "name": "dropped", "transform": "bucket[16]"}
	]}`
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}

	var s icebergTablePartitionSpec
	if err := s.FromIceberg(spec, sourceRefSchema()); err != nil {
		t.Fatalf("FromIceberg: %v", err)
	}

	want := [][]string{{"event_time"}, {"location.lat"}, {"id", "event_time"}, nil}
	for i, names := range want {
		got := s.Fields[i].SourceNames
		if names == nil {
			// One unresolvable id nulls the whole list.
			if !got.IsNull() {
				t.Errorf("field %d: expected null source names, got %v", i, got)
			}

			continue
		}
		if g := stringsOf(t, got); !slices.Equal(g, names) {
			t.Errorf("field %d: expected source names %v, got %v", i, names, g)
		}
	}
	if got := int64sOf(t, s.Fields[3].SourceIDs); !slices.Equal(got, []int64{1, 99}) {
		t.Errorf("expected source ids [1 99] for unresolvable field, got %v", got)
	}
}

func TestSortOrderFromIcebergSourceName(t *testing.T) {
	var order table.SortOrder
	orderJSON := `{"order-id": 1, "fields": [
		{"source-id": 2, "transform": "identity", "direction": "asc", "null-order": "nulls-first"},
		{"source-id": 5, "transform": "identity", "direction": "desc", "null-order": "nulls-last"},
		{"source-id": 99, "transform": "identity", "direction": "asc", "null-order": "nulls-first"}
	]}`
	if err := json.Unmarshal([]byte(orderJSON), &order); err != nil {
		t.Fatalf("unmarshal sort order: %v", err)
	}

	var s icebergTableSortOrder
	if err := s.FromIceberg(order, sourceRefSchema()); err != nil {
		t.Fatalf("FromIceberg: %v", err)
	}

	wantIDs := []int64{2, 5, 99}
	wantNames := []types.String{types.StringValue("event_time"), types.StringValue("location.lat"), types.StringNull()}
	for i := range wantIDs {
		if got := s.Fields[i].SourceID; !got.Equal(types.Int64Value(wantIDs[i])) {
			t.Errorf("field %d: expected source id %d, got %v", i, wantIDs[i], got)
		}
		if got := s.Fields[i].SourceName; !got.Equal(wantNames[i]) {
			t.Errorf("field %d: expected source name %v, got %v", i, wantNames[i], got)
		}
	}
}

func TestPartitionSpecJSONOmitsSourceNames(t *testing.T) {
	spec := icebergTablePartitionSpec{
		SpecID: types.Int64Null(),
		Fields: []icebergTablePartitionField{{
			SourceIDs:   int64List(2),
			SourceNames: stringList("event_time"),
			FieldID:     types.Int64Unknown(),
			Name:        "ts_day",
			Transform:   "day",
		}},
	}

	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if s := string(b); strings.Contains(s, "event_time") || strings.Contains(s, "source-name") || !strings.Contains(s, `"source-ids":[2]`) {
		t.Errorf("expected Iceberg JSON with source-ids and no source names, got %s", s)
	}

	got, err := spec.ToIceberg(sourceRefSchema())
	if err != nil {
		t.Fatalf("ToIceberg: %v", err)
	}
	if ids := got.Field(0).SourceIDs; !slices.Equal(ids, []int{2}) {
		t.Errorf("expected source ids [2], got %v", ids)
	}
}

func TestSortOrderJSONOmitsSourceName(t *testing.T) {
	order := icebergTableSortOrder{
		OrderID: types.Int64Null(),
		Fields: []icebergTableSortField{{
			SourceID:   types.Int64Value(2),
			SourceName: types.StringValue("event_time"),
			Transform:  "identity",
			Direction:  "desc",
			NullOrder:  "nulls-last",
		}},
	}

	b, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if s := string(b); strings.Contains(s, "event_time") || strings.Contains(s, "source-name") || !strings.Contains(s, `"source-id":2`) {
		t.Errorf("expected Iceberg JSON with source-id and no source name, got %s", s)
	}

	got, err := order.ToIceberg(sourceRefSchema())
	if err != nil {
		t.Fatalf("ToIceberg: %v", err)
	}
	if f := got.Field(0); f.SourceID() != 2 || f.Direction != table.SortDESC || f.NullOrder != table.NullsLast {
		t.Errorf("unexpected sort field after conversion: %+v", f)
	}

	var back icebergTableSortOrder
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f := back.Fields[0]; !f.SourceID.Equal(types.Int64Value(2)) || !f.SourceName.IsNull() {
		t.Errorf("expected source id 2 and null source name after round trip, got %+v", f)
	}
}

func TestToIcebergResolvesSourceNames(t *testing.T) {
	spec := icebergTablePartitionSpec{Fields: []icebergTablePartitionField{partitionFieldByName("event_time")}}
	gotSpec, err := spec.ToIceberg(sourceRefSchema())
	if err != nil {
		t.Fatalf("partition ToIceberg: %v", err)
	}
	if ids := gotSpec.Field(0).SourceIDs; !slices.Equal(ids, []int{2}) {
		t.Errorf("expected partition source ids [2], got %v", ids)
	}
	if !spec.Fields[0].SourceIDs.IsNull() {
		t.Errorf("expected ToIceberg to leave the partition spec unchanged, got source_ids %v", spec.Fields[0].SourceIDs)
	}

	order := icebergTableSortOrder{Fields: []icebergTableSortField{sortFieldByName("event_time")}}
	gotOrder, err := order.ToIceberg(sourceRefSchema())
	if err != nil {
		t.Fatalf("sort ToIceberg: %v", err)
	}
	if id := gotOrder.Field(0).SourceID(); id != 2 {
		t.Errorf("expected sort source id 2, got %d", id)
	}
	if !order.Fields[0].SourceID.IsNull() {
		t.Errorf("expected ToIceberg to leave the sort order unchanged, got source_id %v", order.Fields[0].SourceID)
	}
}

func TestToIcebergRejectsMissingSources(t *testing.T) {
	noSource := partitionFieldByName()
	noSource.SourceNames = types.ListNull(types.StringType)
	spec := icebergTablePartitionSpec{Fields: []icebergTablePartitionField{noSource}}
	if _, err := spec.ToIceberg(sourceRefSchema()); err == nil || !strings.Contains(err.Error(), `partition field "p" has neither source_ids nor source_names`) {
		t.Errorf("expected missing partition source error, got %v", err)
	}

	noSortSource := sortFieldByName("")
	noSortSource.SourceName = types.StringNull()
	order := icebergTableSortOrder{Fields: []icebergTableSortField{noSortSource}}
	if _, err := order.ToIceberg(sourceRefSchema()); err == nil || !strings.Contains(err.Error(), "sort_order.fields[0] has neither source_id nor source_name") {
		t.Errorf("expected missing sort source error, got %v", err)
	}
}
