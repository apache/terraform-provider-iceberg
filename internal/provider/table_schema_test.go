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
	"strings"
	"testing"

	"github.com/apache/iceberg-go"
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
