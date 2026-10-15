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
	"slices"
	"testing"

	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
)

func testTableMetadata(t *testing.T) table.Metadata {
	t.Helper()

	schema := iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
	)
	spec := iceberg.NewPartitionSpec(
		iceberg.PartitionField{SourceIDs: []int{2}, FieldID: 1000, Name: "data", Transform: iceberg.IdentityTransform{}},
	)
	meta, err := table.NewMetadata(schema, &spec, table.UnsortedSortOrder, "s3://bucket/table", nil)
	if err != nil {
		t.Fatalf("NewMetadata: %v", err)
	}

	return meta
}

func requirementTypes(reqs []table.Requirement) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.GetType()
	}

	return out
}

func TestCommitRequirements_PropertiesOnly(t *testing.T) {
	meta := testTableMetadata(t)
	updates := []table.Update{
		table.NewSetPropertiesUpdate(iceberg.Properties{"owner": "alice"}),
	}

	got := requirementTypes(commitRequirements(meta, updates))
	want := []string{"assert-table-uuid"}
	if !slices.Equal(got, want) {
		t.Fatalf("requirements = %v, want %v", got, want)
	}
}

func TestCommitRequirements_SchemaUpdates(t *testing.T) {
	meta := testTableMetadata(t)
	newSchema := iceberg.NewSchema(1,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
		iceberg.NestedField{ID: 3, Name: "extra", Type: iceberg.PrimitiveTypes.String, Required: false},
	)
	updates := []table.Update{
		table.NewAddSchemaUpdate(newSchema),
		table.NewSetCurrentSchemaUpdate(1),
		// Duplicate actions must not duplicate requirements.
		table.NewAddSchemaUpdate(newSchema),
		table.NewSetCurrentSchemaUpdate(1),
	}

	got := requirementTypes(commitRequirements(meta, updates))
	want := []string{
		"assert-table-uuid",
		"assert-last-assigned-field-id",
		"assert-current-schema-id",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("requirements = %v, want %v", got, want)
	}
}

func TestCommitRequirements_PartitionUpdates(t *testing.T) {
	meta := testTableMetadata(t)
	unpartitioned := iceberg.NewPartitionSpec()
	updates := []table.Update{
		table.NewAddPartitionSpecUpdate(&unpartitioned, false),
		table.NewSetDefaultSpecUpdate(-1),
		table.NewAddPartitionSpecUpdate(&unpartitioned, false),
		table.NewSetDefaultSpecUpdate(-1),
	}

	got := requirementTypes(commitRequirements(meta, updates))
	want := []string{
		"assert-table-uuid",
		"assert-last-assigned-partition-id",
		"assert-default-spec-id",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("requirements = %v, want %v", got, want)
	}
}

func TestCommitRequirements_SortOrderUpdates(t *testing.T) {
	meta := testTableMetadata(t)
	order, err := table.NewSortOrder(1, []table.SortField{
		{SourceIDs: []int{1}, Transform: iceberg.IdentityTransform{}, Direction: table.SortASC, NullOrder: table.NullsFirst},
	})
	if err != nil {
		t.Fatalf("NewSortOrder: %v", err)
	}
	updates := []table.Update{
		table.NewAddSortOrderUpdate(&order),
		table.NewSetDefaultSortOrderUpdate(-1),
		table.NewSetDefaultSortOrderUpdate(1),
	}

	got := requirementTypes(commitRequirements(meta, updates))
	want := []string{
		"assert-table-uuid",
		"assert-default-sort-order-id",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("requirements = %v, want %v", got, want)
	}
}

func TestCommitRequirements_CombinedSchemaAndPartition(t *testing.T) {
	meta := testTableMetadata(t)
	newSchema := iceberg.NewSchema(1,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
		iceberg.NestedField{ID: 3, Name: "extra", Type: iceberg.PrimitiveTypes.String, Required: false},
	)
	unpartitioned := iceberg.NewPartitionSpec()
	updates := []table.Update{
		table.NewAddSchemaUpdate(newSchema),
		table.NewSetCurrentSchemaUpdate(1),
		table.NewAddPartitionSpecUpdate(&unpartitioned, false),
		table.NewSetDefaultSpecUpdate(-1),
	}

	got := requirementTypes(commitRequirements(meta, updates))
	want := []string{
		"assert-table-uuid",
		"assert-last-assigned-field-id",
		"assert-current-schema-id",
		"assert-last-assigned-partition-id",
		"assert-default-spec-id",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("requirements = %v, want %v", got, want)
	}
}

func TestCommitRequirements_ValidateAgainstBaseMetadata(t *testing.T) {
	meta := testTableMetadata(t)
	newSchema := iceberg.NewSchema(1,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
		iceberg.NestedField{ID: 3, Name: "extra", Type: iceberg.PrimitiveTypes.String, Required: false},
	)
	unpartitioned := iceberg.NewPartitionSpec()
	order, err := table.NewSortOrder(1, []table.SortField{
		{SourceIDs: []int{1}, Transform: iceberg.IdentityTransform{}, Direction: table.SortASC, NullOrder: table.NullsFirst},
	})
	if err != nil {
		t.Fatalf("NewSortOrder: %v", err)
	}

	updates := []table.Update{
		table.NewAddSchemaUpdate(newSchema),
		table.NewSetCurrentSchemaUpdate(1),
		table.NewAddPartitionSpecUpdate(&unpartitioned, false),
		table.NewSetDefaultSpecUpdate(-1),
		table.NewAddSortOrderUpdate(&order),
		table.NewSetDefaultSortOrderUpdate(-1),
	}

	for _, req := range commitRequirements(meta, updates) {
		if err := req.Validate(meta); err != nil {
			t.Fatalf("%s should pass against base metadata: %v", req.GetType(), err)
		}
	}
}

func TestCommitRequirements_ConflictOnConcurrentSchemaChange(t *testing.T) {
	meta := testTableMetadata(t)
	newSchema := iceberg.NewSchema(1,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
		iceberg.NestedField{ID: 3, Name: "extra", Type: iceberg.PrimitiveTypes.String, Required: false},
	)
	updates := []table.Update{
		table.NewAddSchemaUpdate(newSchema),
		table.NewSetCurrentSchemaUpdate(1),
	}
	reqs := commitRequirements(meta, updates)

	// Simulate a concurrent client committing a schema change first.
	builder, err := table.MetadataBuilderFromBase(meta, "")
	if err != nil {
		t.Fatalf("MetadataBuilderFromBase: %v", err)
	}
	concurrentSchema := iceberg.NewSchema(1,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "data", Type: iceberg.PrimitiveTypes.String, Required: false},
		iceberg.NestedField{ID: 3, Name: "other", Type: iceberg.PrimitiveTypes.Int32, Required: false},
	)
	if err := builder.AddSchema(concurrentSchema); err != nil {
		t.Fatalf("AddSchema: %v", err)
	}
	if err := builder.SetCurrentSchemaID(-1); err != nil {
		t.Fatalf("SetCurrentSchemaID: %v", err)
	}
	conflictMeta, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var sawConflict bool
	for _, req := range reqs {
		if err := req.Validate(conflictMeta); err != nil {
			sawConflict = true
			t.Logf("expected conflict from %s: %v", req.GetType(), err)
		}
	}
	if !sawConflict {
		t.Fatal("expected stale schema requirements to fail after a concurrent schema commit")
	}
}

func TestCommitRequirements_ConflictOnConcurrentPartitionChange(t *testing.T) {
	meta := testTableMetadata(t)
	unpartitioned := iceberg.NewPartitionSpec()
	updates := []table.Update{
		table.NewAddPartitionSpecUpdate(&unpartitioned, false),
		table.NewSetDefaultSpecUpdate(-1),
	}
	reqs := commitRequirements(meta, updates)

	builder, err := table.MetadataBuilderFromBase(meta, "")
	if err != nil {
		t.Fatalf("MetadataBuilderFromBase: %v", err)
	}
	// Concurrent client adds a different partition field (new last-assigned partition ID).
	newSpec := iceberg.NewPartitionSpec(
		iceberg.PartitionField{SourceIDs: []int{1}, FieldID: 1001, Name: "id", Transform: iceberg.IdentityTransform{}},
	)
	if err := builder.AddPartitionSpec(&newSpec, false); err != nil {
		t.Fatalf("AddPartitionSpec: %v", err)
	}
	if err := builder.SetDefaultSpecID(-1); err != nil {
		t.Fatalf("SetDefaultSpecID: %v", err)
	}
	conflictMeta, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var sawConflict bool
	for _, req := range reqs {
		if err := req.Validate(conflictMeta); err != nil {
			sawConflict = true
			t.Logf("expected conflict from %s: %v", req.GetType(), err)
		}
	}
	if !sawConflict {
		t.Fatal("expected stale partition requirements to fail after a concurrent partition commit")
	}
}
