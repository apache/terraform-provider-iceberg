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

func TestValidateFieldIDParents(t *testing.T) {
	child := iceberg.NestedField{ID: 3, Name: "child", Type: iceberg.PrimitiveTypes.Int64, Required: false}

	twoStructs := func(leftFields, rightFields []iceberg.NestedField) *iceberg.Schema {
		return iceberg.NewSchema(0,
			iceberg.NestedField{ID: 1, Name: "left", Type: &iceberg.StructType{FieldList: leftFields}, Required: false},
			iceberg.NestedField{ID: 2, Name: "right", Type: &iceberg.StructType{FieldList: rightFields}, Required: false},
		)
	}

	twoLists := func(leftElemID, rightElemID int) *iceberg.Schema {
		return iceberg.NewSchema(0,
			iceberg.NestedField{ID: 1, Name: "left", Type: &iceberg.ListType{
				ElementID: leftElemID, Element: iceberg.PrimitiveTypes.Int64, ElementRequired: false,
			}, Required: false},
			iceberg.NestedField{ID: 2, Name: "right", Type: &iceberg.ListType{
				ElementID: rightElemID, Element: iceberg.PrimitiveTypes.String, ElementRequired: false,
			}, Required: false},
		)
	}

	twoMaps := func(leftKeyID, leftValueID, rightKeyID, rightValueID int) *iceberg.Schema {
		return iceberg.NewSchema(0,
			iceberg.NestedField{ID: 1, Name: "left", Type: &iceberg.MapType{
				KeyID: leftKeyID, KeyType: iceberg.PrimitiveTypes.String,
				ValueID: leftValueID, ValueType: iceberg.PrimitiveTypes.Int64, ValueRequired: false,
			}, Required: false},
			iceberg.NestedField{ID: 2, Name: "right", Type: &iceberg.MapType{
				KeyID: rightKeyID, KeyType: iceberg.PrimitiveTypes.String,
				ValueID: rightValueID, ValueType: iceberg.PrimitiveTypes.Int32, ValueRequired: false,
			}, Required: false},
		)
	}

	// list<map<string, struct<item: long>>> under left and right branches.
	deepListMapStruct := func(leftItemID, rightItemID int) *iceberg.Schema {
		branch := func(listID, elemID, keyID, valueID, itemID int, name string) iceberg.NestedField {
			return iceberg.NestedField{ID: listID, Name: name, Type: &iceberg.ListType{
				ElementID: elemID,
				Element: &iceberg.MapType{
					KeyID: keyID, KeyType: iceberg.PrimitiveTypes.String,
					ValueID: valueID,
					ValueType: &iceberg.StructType{FieldList: []iceberg.NestedField{
						{ID: itemID, Name: "item", Type: iceberg.PrimitiveTypes.Int64, Required: false},
					}},
					ValueRequired: false,
				},
				ElementRequired: false,
			}, Required: false}
		}

		return iceberg.NewSchema(0,
			branch(1, 3, 4, 5, leftItemID, "left"),
			branch(2, 7, 8, 9, rightItemID, "right"),
		)
	}

	tests := []struct {
		name    string
		current *iceberg.Schema
		planned *iceberg.Schema
		wantErr string
	}{
		{
			name:    "struct child stays under same parent",
			current: twoStructs([]iceberg.NestedField{child}, nil),
			planned: twoStructs([]iceberg.NestedField{child}, nil),
		},
		{
			name:    "struct child renamed under same parent",
			current: twoStructs([]iceberg.NestedField{child}, nil),
			planned: twoStructs([]iceberg.NestedField{{ID: 3, Name: "renamed", Type: iceberg.PrimitiveTypes.Int64, Required: false}}, nil),
		},
		{
			name:    "struct child dropped and new id added under other parent",
			current: twoStructs([]iceberg.NestedField{child}, nil),
			planned: twoStructs(nil, []iceberg.NestedField{{ID: 4, Name: "child", Type: iceberg.PrimitiveTypes.Int64, Required: false}}),
		},
		{
			name:    "struct child moved between parents",
			current: twoStructs([]iceberg.NestedField{child}, nil),
			planned: twoStructs(nil, []iceberg.NestedField{child}),
			wantErr: `field "child" (id 3): cannot move from parent id 1 ("left") to parent id 2 ("right")`,
		},
		{
			name:    "list element moved between parents",
			current: twoLists(3, 4),
			planned: twoLists(5, 3),
			wantErr: `field "element" (id 3): cannot move from parent id 1 ("left") to parent id 2 ("right")`,
		},
		{
			name:    "map key moved between parents",
			current: twoMaps(3, 4, 5, 6),
			planned: twoMaps(7, 4, 3, 6),
			wantErr: `field "key" (id 3): cannot move from parent id 1 ("left") to parent id 2 ("right")`,
		},
		{
			name:    "map value moved between parents",
			current: twoMaps(3, 4, 5, 6),
			planned: twoMaps(3, 7, 5, 4),
			wantErr: `field "value" (id 4): cannot move from parent id 1 ("left") to parent id 2 ("right")`,
		},
		{
			name:    "map key and value stay under same map parent",
			current: twoMaps(3, 4, 5, 6),
			planned: twoMaps(3, 4, 5, 6),
		},
		{
			name: "nested fields reordered under same map-value struct parent",
			current: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "m", Type: &iceberg.MapType{
					KeyID: 2, KeyType: iceberg.PrimitiveTypes.String,
					ValueID: 3, ValueType: &iceberg.StructType{FieldList: []iceberg.NestedField{
						{ID: 4, Name: "a", Type: iceberg.PrimitiveTypes.Int64, Required: false},
						{ID: 5, Name: "b", Type: iceberg.PrimitiveTypes.String, Required: false},
					}}, ValueRequired: false,
				}, Required: false},
			),
			planned: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "m", Type: &iceberg.MapType{
					KeyID: 2, KeyType: iceberg.PrimitiveTypes.String,
					ValueID: 3, ValueType: &iceberg.StructType{FieldList: []iceberg.NestedField{
						{ID: 5, Name: "b", Type: iceberg.PrimitiveTypes.String, Required: false},
						{ID: 4, Name: "a", Type: iceberg.PrimitiveTypes.Int64, Required: false},
					}}, ValueRequired: false,
				}, Required: false},
			),
		},
		{
			name: "root field moved into a struct",
			current: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "wrapper", Type: &iceberg.StructType{}, Required: false},
				iceberg.NestedField{ID: 3, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
			),
			planned: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "wrapper", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
					{ID: 3, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
				}}, Required: false},
			),
			wantErr: `field "data" (id 3): cannot move from parent table root to parent id 1 ("wrapper")`,
		},
		{
			name: "nested field moved to table root",
			current: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "wrapper", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
					{ID: 3, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
				}}, Required: false},
			),
			planned: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "wrapper", Type: &iceberg.StructType{}, Required: false},
				iceberg.NestedField{ID: 3, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
			),
			wantErr: `field "data" (id 3): cannot move from parent id 1 ("wrapper") to parent table root`,
		},
		{
			name: "rename swap of parents that also moves children",
			// Swapping a/b by renaming and reparenting yields the same parent
			// name on both sides; the message must still show distinct ids.
			current: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "a", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
					{ID: 3, Name: "x", Type: iceberg.PrimitiveTypes.Int64, Required: false},
				}}, Required: false},
				iceberg.NestedField{ID: 2, Name: "b", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
					{ID: 4, Name: "x", Type: iceberg.PrimitiveTypes.Int64, Required: false},
				}}, Required: false},
			),
			planned: iceberg.NewSchema(0,
				iceberg.NestedField{ID: 2, Name: "a", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
					{ID: 3, Name: "x", Type: iceberg.PrimitiveTypes.Int64, Required: false},
				}}, Required: false},
				iceberg.NestedField{ID: 1, Name: "b", Type: &iceberg.StructType{FieldList: []iceberg.NestedField{
					{ID: 4, Name: "x", Type: iceberg.PrimitiveTypes.Int64, Required: false},
				}}, Required: false},
			),
			wantErr: `field "x" (id 3): cannot move from parent id 1 ("a") to parent id 2 ("a")`,
		},
		{
			name: "deep nested list-of-map-of-struct child stays",
			current: deepListMapStruct(6, 10),
			planned: deepListMapStruct(6, 10),
		},
		{
			name:    "deep nested list-of-map-of-struct child moved between branches",
			current: deepListMapStruct(6, 10),
			planned: deepListMapStruct(10, 6),
			wantErr: `field "item" (id 6): cannot move from parent id 5 ("value") to parent id 9 ("value")`,
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

// TestValidateFieldIDParentsOmittedConfigIDs covers the update path where
// Terraform config leaves field ids unset and resolveFieldIDs restores them
// from prior state before parent validation runs.
func TestValidateFieldIDParentsOmittedConfigIDs(t *testing.T) {
	prior := icebergTableSchema{Fields: []icebergTableSchemaField{
		{
			ID: types.Int64Value(1), Name: "left", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(3), Name: "child", Type: "long", Required: false},
			}},
		},
		{
			ID: types.Int64Value(2), Name: "right", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(4), Name: "keep", Type: "string", Required: false},
			}},
		},
	}}

	planStable := icebergTableSchema{Fields: []icebergTableSchemaField{
		{
			ID: types.Int64Null(), Name: "left", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: []icebergTableSchemaField{
				{ID: types.Int64Null(), Name: "child", Type: "long", Required: false},
			}},
		},
		{
			ID: types.Int64Null(), Name: "right", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: []icebergTableSchemaField{
				{ID: types.Int64Null(), Name: "keep", Type: "string", Required: false},
			}},
		},
	}}
	planStable.resolveFieldIDs(&prior)
	planStable.assignFieldIDs(4)

	priorIceberg, err := prior.ToIceberg()
	if err != nil {
		t.Fatalf("prior ToIceberg: %v", err)
	}
	stableIceberg, err := planStable.ToIceberg()
	if err != nil {
		t.Fatalf("stable plan ToIceberg: %v", err)
	}
	if err := validateSchemaEvolution(priorIceberg, stableIceberg); err != nil {
		t.Fatalf("omitted ids on unchanged nested schema: %v", err)
	}
	if planStable.Fields[0].StructProperties.Fields[0].ID.ValueInt64() != 3 {
		t.Fatalf("expected child id resolved to 3, got %v", planStable.Fields[0].StructProperties.Fields[0].ID)
	}

	// Moving "child" under right without an explicit id assigns a fresh id
	// (path /right/child was not in prior), which is drop+add — allowed.
	planMoveByName := icebergTableSchema{Fields: []icebergTableSchemaField{
		{
			ID: types.Int64Null(), Name: "left", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: nil},
		},
		{
			ID: types.Int64Null(), Name: "right", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: []icebergTableSchemaField{
				{ID: types.Int64Null(), Name: "keep", Type: "string", Required: false},
				{ID: types.Int64Null(), Name: "child", Type: "long", Required: false},
			}},
		},
	}}
	planMoveByName.resolveFieldIDs(&prior)
	planMoveByName.assignFieldIDs(4)
	movedIceberg, err := planMoveByName.ToIceberg()
	if err != nil {
		t.Fatalf("moved plan ToIceberg: %v", err)
	}
	if err := validateSchemaEvolution(priorIceberg, movedIceberg); err != nil {
		t.Fatalf("omitted-id move-by-name should be drop+add, got %v", err)
	}
	childID := planMoveByName.Fields[1].StructProperties.Fields[1].ID.ValueInt64()
	if childID == 3 {
		t.Fatalf("expected fresh id for /right/child, still got prior id 3")
	}

	// Keeping the prior child id while placing it under right must still fail.
	planReparentExplicit := icebergTableSchema{Fields: []icebergTableSchemaField{
		{
			ID: types.Int64Null(), Name: "left", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: nil},
		},
		{
			ID: types.Int64Null(), Name: "right", Type: "struct", Required: false,
			StructProperties: &icebergTableSchemaFieldStructProperties{Fields: []icebergTableSchemaField{
				{ID: types.Int64Null(), Name: "keep", Type: "string", Required: false},
				{ID: types.Int64Value(3), Name: "child", Type: "long", Required: false},
			}},
		},
	}}
	planReparentExplicit.resolveFieldIDs(&prior)
	planReparentExplicit.assignFieldIDs(4)
	reparentIceberg, err := planReparentExplicit.ToIceberg()
	if err != nil {
		t.Fatalf("reparent plan ToIceberg: %v", err)
	}
	err = validateSchemaEvolution(priorIceberg, reparentIceberg)
	if err == nil {
		t.Fatal("expected reparent with explicit id to fail")
	}
	want := `field "child" (id 3): cannot move from parent id 1 ("left") to parent id 2 ("right")`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
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
			if err := s.FromIceberg(want); err != nil {
				t.Fatalf("FromIceberg: %v", err)
			}
			if got := s.Fields[0].SourceIDs; !slices.Equal(got, tt.sourceIDs) {
				t.Errorf("expected source ids %v, got %v", tt.sourceIDs, got)
			}

			got, err := s.ToIceberg()
			if err != nil {
				t.Fatalf("ToIceberg: %v", err)
			}
			if !got.Equals(want) {
				t.Errorf("expected spec %v after round trip, got %v", want, *got)
			}
		})
	}
}
