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

func TestValidateNewFieldIDs(t *testing.T) {
	// last-column-id 5 is above any id a schema used.
	metadata, err := table.ParseMetadataString(`{
		"format-version": 2,
		"table-uuid": "9c12d441-03fe-4693-9a96-a0705ddf69c1",
		"location": "s3://warehouse/table",
		"last-sequence-number": 0,
		"last-updated-ms": 1700000000000,
		"last-column-id": 5,
		"current-schema-id": 1,
		"schemas": [
			{"type": "struct", "schema-id": 0, "fields": [
				{"id": 1, "name": "id", "required": true, "type": "long"},
				{"id": 2, "name": "dropped", "required": false, "type": "string"},
				{"id": 3, "name": "tags", "required": false, "type": {
					"type": "list", "element-id": 4, "element": "string", "element-required": false}}
			]},
			{"type": "struct", "schema-id": 1, "fields": [
				{"id": 1, "name": "id", "required": true, "type": "long"}
			]}
		],
		"default-spec-id": 0,
		"partition-specs": [{"spec-id": 0, "fields": []}],
		"last-partition-id": 999,
		"default-sort-order-id": 0,
		"sort-orders": [{"order-id": 0, "fields": []}]
	}`)
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}

	id := iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true}
	withField := func(f iceberg.NestedField) *iceberg.Schema {
		return iceberg.NewSchema(2, id, f)
	}

	tests := []struct {
		name    string
		planned *iceberg.Schema
		wantErr string
	}{
		{name: "unchanged", planned: iceberg.NewSchema(2, id)},
		{
			name:    "new id above last-column-id",
			planned: withField(iceberg.NestedField{ID: 6, Name: "added", Type: iceberg.PrimitiveTypes.String}),
		},
		{
			name:    "dropped field id",
			planned: withField(iceberg.NestedField{ID: 2, Name: "replacement", Type: iceberg.PrimitiveTypes.String}),
			wantErr: `field "replacement" has id 2, which belonged to dropped field "dropped"`,
		},
		{
			name:    "dropped list element id",
			planned: withField(iceberg.NestedField{ID: 4, Name: "replacement", Type: iceberg.PrimitiveTypes.String}),
			wantErr: `which belonged to dropped field "tags.element"`,
		},
		{
			name:    "assigned id never in a schema",
			planned: withField(iceberg.NestedField{ID: 5, Name: "replacement", Type: iceberg.PrimitiveTypes.String}),
			wantErr: "new fields need ids above the last-column-id 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNewFieldIDs(metadata, tt.planned)
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

func TestSchemaIdentifierFieldsRoundTrip(t *testing.T) {
	icebergSchema := iceberg.NewSchemaWithIdentifiers(0, []int{1},
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
	)

	var tfSchema icebergTableSchema
	if err := tfSchema.FromIceberg(icebergSchema); err != nil {
		t.Fatalf("FromIceberg: %v", err)
	}

	names, err := stringSetElements(tfSchema.IdentifierFields)
	if err != nil {
		t.Fatalf("stringSetElements: %v", err)
	}
	if len(names) != 1 || names[0] != "id" {
		t.Fatalf("expected identifier_fields [id], got %v", names)
	}

	roundTripped, err := tfSchema.ToIceberg()
	if err != nil {
		t.Fatalf("ToIceberg: %v", err)
	}
	if len(roundTripped.IdentifierFieldIDs) != 1 || roundTripped.IdentifierFieldIDs[0] != 1 {
		t.Fatalf("expected round-trip identifier-field-ids [1], got %v", roundTripped.IdentifierFieldIDs)
	}
}

func TestPreserveIdentifierFields(t *testing.T) {
	current := iceberg.NewSchemaWithIdentifiers(0, []int{1},
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
	)

	t.Run("omitted plan preserves current", func(t *testing.T) {
		plan := icebergTableSchema{
			IdentifierFields: types.SetNull(types.StringType),
			Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(1), Name: "id", Type: "long", Required: true},
			},
		}
		plan.preserveIdentifierFields(current)
		names, err := stringSetElements(plan.IdentifierFields)
		if err != nil {
			t.Fatalf("stringSetElements: %v", err)
		}
		if len(names) != 1 || names[0] != "id" {
			t.Fatalf("expected preserved [id], got %v", names)
		}
	})

	t.Run("explicit empty set clears", func(t *testing.T) {
		plan := icebergTableSchema{
			IdentifierFields: types.SetValueMust(types.StringType, []attr.Value{}),
			Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(1), Name: "id", Type: "long", Required: true},
			},
		}
		plan.preserveIdentifierFields(current)
		names, err := stringSetElements(plan.IdentifierFields)
		if err != nil {
			t.Fatalf("stringSetElements: %v", err)
		}
		if len(names) != 0 {
			t.Fatalf("expected cleared identifiers, got %v", names)
		}
	})

	t.Run("explicit plan set kept", func(t *testing.T) {
		plan := icebergTableSchema{
			IdentifierFields: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("id")}),
			Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(1), Name: "id", Type: "long", Required: true},
				{ID: types.Int64Value(2), Name: "data", Type: "string", Required: false},
			},
		}
		plan.preserveIdentifierFields(current)
		names, err := stringSetElements(plan.IdentifierFields)
		if err != nil {
			t.Fatalf("stringSetElements: %v", err)
		}
		if len(names) != 1 || names[0] != "id" {
			t.Fatalf("expected plan [id], got %v", names)
		}
	})

	t.Run("rename remaps preserved identifier by id", func(t *testing.T) {
		plan := icebergTableSchema{
			IdentifierFields: types.SetNull(types.StringType),
			Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(1), Name: "user_id", Type: "long", Required: true},
			},
		}
		plan.preserveIdentifierFields(current)
		names, err := stringSetElements(plan.IdentifierFields)
		if err != nil {
			t.Fatalf("stringSetElements: %v", err)
		}
		if len(names) != 1 || names[0] != "user_id" {
			t.Fatalf("expected preserved rename [user_id], got %v", names)
		}
	})

	t.Run("dropped identifier column is not preserved", func(t *testing.T) {
		plan := icebergTableSchema{
			IdentifierFields: types.SetNull(types.StringType),
			Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(2), Name: "data", Type: "string", Required: false},
			},
		}
		plan.preserveIdentifierFields(current)
		names, err := stringSetElements(plan.IdentifierFields)
		if err != nil {
			t.Fatalf("stringSetElements: %v", err)
		}
		if len(names) != 0 {
			t.Fatalf("expected no preserved identifiers after drop, got %v", names)
		}
	})
}

func TestValidateIdentifierFields(t *testing.T) {
	t.Run("missing field", func(t *testing.T) {
		s := icebergTableSchema{
			IdentifierFields: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("missing")}),
			Fields: []icebergTableSchemaField{
				{ID: types.Int64Value(1), Name: "id", Type: "long", Required: true},
			},
		}
		_, err := s.ToIceberg()
		if err == nil || !strings.Contains(err.Error(), `identifier field "missing" does not exist`) {
			t.Fatalf("expected missing-field error, got %v", err)
		}
	})

	t.Run("optional field rejected", func(t *testing.T) {
		schema := iceberg.NewSchemaWithIdentifiers(0, []int{1},
			iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: false},
		)
		err := validateIdentifierFields(schema)
		if err == nil || !strings.Contains(err.Error(), "not a required field") {
			t.Fatalf("expected required-field error, got %v", err)
		}
	})

	t.Run("float field rejected", func(t *testing.T) {
		schema := iceberg.NewSchemaWithIdentifiers(0, []int{1},
			iceberg.NestedField{ID: 1, Name: "score", Type: iceberg.PrimitiveTypes.Float64, Required: true},
		)
		err := validateIdentifierFields(schema)
		if err == nil || !strings.Contains(err.Error(), "must not be float or double") {
			t.Fatalf("expected float error, got %v", err)
		}
	})
}

