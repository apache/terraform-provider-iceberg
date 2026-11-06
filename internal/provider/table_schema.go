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

package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type icebergTableSchema struct {
	ID     types.Int64               `tfsdk:"id" json:"schema-id"`
	Fields []icebergTableSchemaField `tfsdk:"fields" json:"fields"`
}

func (s icebergTableSchema) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.Int64Type,
		"fields": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: icebergTableSchemaField{}.AttrTypes(4),
			},
		},
	}
}

func (s icebergTableSchema) MarshalJSON() ([]byte, error) {
	type Alias struct {
		ID     *int64                    `json:"schema-id,omitempty"`
		Fields []icebergTableSchemaField `json:"fields"`
	}
	var id *int64
	if !s.ID.IsNull() && !s.ID.IsUnknown() {
		val := s.ID.ValueInt64()
		id = &val
	}

	return json.Marshal(&struct {
		Type string `json:"type"`
		Alias
	}{
		Type: "struct",
		Alias: Alias{
			ID:     id,
			Fields: s.Fields,
		},
	})
}

func (s *icebergTableSchema) UnmarshalJSON(b []byte) error {
	type Alias struct {
		ID     int64                     `json:"schema-id"`
		Fields []icebergTableSchemaField `json:"fields"`
	}
	var raw Alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.ID = types.Int64Value(raw.ID)
	s.Fields = raw.Fields

	return nil
}

func (s *icebergTableSchema) ToIceberg() (*iceberg.Schema, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var icebergSchema iceberg.Schema
	if err := json.Unmarshal(b, &icebergSchema); err != nil {
		return nil, err
	}

	return &icebergSchema, nil
}

func (s *icebergTableSchema) FromIceberg(icebergSchema *iceberg.Schema) error {
	b, err := json.Marshal(icebergSchema)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, s)
}

// fieldIDUnset reports an unspecified field ID: null, unknown, or 0 (Iceberg
// reserves 0 for the table root).
func fieldIDUnset(id types.Int64) bool {
	return id.IsNull() || id.IsUnknown() || id.ValueInt64() == 0
}

// assignFieldIDs fills unset IDs (nested struct/list/map included) with fresh
// values above startAfter and every existing ID, preserving user-set ones. Pass
// the table's last-column-id on update so new IDs don't reuse retired columns'.
// Fresh IDs are unique by construction; user-set ones need validateFieldIDs.
func (s *icebergTableSchema) assignFieldIDs(startAfter int64) {
	maxID := startAfter

	var scan func(fields []icebergTableSchemaField)
	scan = func(fields []icebergTableSchemaField) {
		for _, f := range fields {
			maxID = maxSetID(maxID, f.ID)
			if f.ListProperties != nil {
				maxID = maxSetID(maxID, f.ListProperties.ID)
			}
			if f.MapProperties != nil {
				maxID = maxSetID(maxID, f.MapProperties.KeyID)
				maxID = maxSetID(maxID, f.MapProperties.ValueID)
			}
			if f.StructProperties != nil {
				scan(f.StructProperties.Fields)
			}
		}
	}
	scan(s.Fields)

	next := func() types.Int64 {
		maxID++

		return types.Int64Value(maxID)
	}

	var assign func(fields []icebergTableSchemaField)
	assign = func(fields []icebergTableSchemaField) {
		for i := range fields {
			f := &fields[i]
			if fieldIDUnset(f.ID) {
				f.ID = next()
			}
			if f.ListProperties != nil && fieldIDUnset(f.ListProperties.ID) {
				f.ListProperties.ID = next()
			}
			if f.MapProperties != nil {
				if fieldIDUnset(f.MapProperties.KeyID) {
					f.MapProperties.KeyID = next()
				}
				if fieldIDUnset(f.MapProperties.ValueID) {
					f.MapProperties.ValueID = next()
				}
			}
			if f.StructProperties != nil {
				assign(f.StructProperties.Fields)
			}
		}
	}
	assign(s.Fields)
}

func maxSetID(current int64, id types.Int64) int64 {
	if fieldIDUnset(id) {
		return current
	}
	if v := id.ValueInt64(); v > current {
		return v
	}

	return current
}

// fieldIDs holds one field's IDs: the field plus any list/map element/key/value.
type fieldIDs struct {
	id      types.Int64
	element types.Int64
	key     types.Int64
	value   types.Int64
}

// resolveFieldIDs reuses prior IDs for same-named fields (matched by path, so
// nested fields stay scoped to their parent), keeping IDs stable across
// inserts/reorders. Fills only unset IDs.
func (s *icebergTableSchema) resolveFieldIDs(prior *icebergTableSchema) {
	byPath := map[string]fieldIDs{}

	var index func(path string, fields []icebergTableSchemaField)
	index = func(path string, fields []icebergTableSchemaField) {
		for _, f := range fields {
			key := path + "/" + f.Name
			ids := fieldIDs{id: f.ID}
			if f.ListProperties != nil {
				ids.element = f.ListProperties.ID
			}
			if f.MapProperties != nil {
				ids.key = f.MapProperties.KeyID
				ids.value = f.MapProperties.ValueID
			}
			byPath[key] = ids
			if f.StructProperties != nil {
				index(key, f.StructProperties.Fields)
			}
		}
	}
	index("", prior.Fields)

	var apply func(path string, fields []icebergTableSchemaField)
	apply = func(path string, fields []icebergTableSchemaField) {
		for i := range fields {
			f := &fields[i]
			key := path + "/" + f.Name
			if match, ok := byPath[key]; ok {
				if fieldIDUnset(f.ID) {
					f.ID = match.id
				}
				if f.ListProperties != nil && fieldIDUnset(f.ListProperties.ID) {
					f.ListProperties.ID = match.element
				}
				if f.MapProperties != nil {
					if fieldIDUnset(f.MapProperties.KeyID) {
						f.MapProperties.KeyID = match.key
					}
					if fieldIDUnset(f.MapProperties.ValueID) {
						f.MapProperties.ValueID = match.value
					}
				}
			}
			if f.StructProperties != nil {
				apply(key, f.StructProperties.Fields)
			}
		}
	}
	apply("", s.Fields)
}

// maxFieldID is the largest field ID a table schema may use.
const maxFieldID = 2147483447

// validateFieldIDs rejects an id outside the usable range or used by more than
// one field. Field, list element, map key/value, and nested ids share one
// space; users may set them, so bad ids must fail before commit.
func (s *icebergTableSchema) validateFieldIDs() error {
	seen := map[int64]string{}

	var walk func(path string, fields []icebergTableSchemaField) error
	walk = func(path string, fields []icebergTableSchemaField) error {
		for _, f := range fields {
			name := path + f.Name
			if err := claimFieldID(seen, f.ID, name); err != nil {
				return err
			}
			if f.ListProperties != nil {
				if err := claimFieldID(seen, f.ListProperties.ID, name+".element"); err != nil {
					return err
				}
			}
			if f.MapProperties != nil {
				if err := claimFieldID(seen, f.MapProperties.KeyID, name+".key"); err != nil {
					return err
				}
				if err := claimFieldID(seen, f.MapProperties.ValueID, name+".value"); err != nil {
					return err
				}
			}
			if f.StructProperties != nil {
				if err := walk(name+".", f.StructProperties.Fields); err != nil {
					return err
				}
			}
		}

		return nil
	}

	return walk("", s.Fields)
}

func claimFieldID(seen map[int64]string, id types.Int64, name string) error {
	if fieldIDUnset(id) {
		return nil
	}
	v := id.ValueInt64()
	if v < 0 || v > maxFieldID {
		return fmt.Errorf("field %q has id %d; field ids must be between 1 and %d (higher ids are reserved by the Iceberg spec)", name, v, maxFieldID)
	}
	if prev, ok := seen[v]; ok {
		return fmt.Errorf("field %q reuses id %d already assigned to %q", name, v, prev)
	}
	seen[v] = name

	return nil
}

// validateSchemaEvolution returns an error if a field present in both schemas
// changes in a way Iceberg disallows. Fields are matched by ID, so additions
// and deletions are always permitted.
func validateSchemaEvolution(current, planned *iceberg.Schema) error {
	currentByID, err := iceberg.IndexByID(current)
	if err != nil {
		return err
	}
	plannedByID, err := iceberg.IndexByID(planned)
	if err != nil {
		return err
	}

	for id, cur := range currentByID {
		if plan, ok := plannedByID[id]; ok {
			if err := validateFieldEvolution(cur, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateFieldEvolution(cur, plan iceberg.NestedField) error {
	if !cur.Required && plan.Required {
		return fmt.Errorf("field %q (id %d): changing a field from optional to required is not an allowed schema evolution", cur.Name, cur.ID)
	}

	curPrim, curIsPrim := cur.Type.(iceberg.PrimitiveType)
	planPrim, planIsPrim := plan.Type.(iceberg.PrimitiveType)

	if !curIsPrim || !planIsPrim {
		// Nested fields carry their own IDs and are checked separately, so only
		// the container kind matters here.
		if cur.Type.Type() != plan.Type.Type() {
			return fmt.Errorf("field %q (id %d): cannot change field type from %s to %s", cur.Name, cur.ID, cur.Type, plan.Type)
		}

		return nil
	}

	// PromoteType rejects a type as a promotion of itself, so only call it when
	// the type actually changed.
	if !planPrim.Equals(curPrim) {
		if _, err := iceberg.PromoteType(curPrim, planPrim); err != nil {
			return fmt.Errorf("field %q (id %d): %s is not a valid type promotion from %s: %w", cur.Name, cur.ID, plan.Type, cur.Type, err)
		}
	}

	return nil
}

type icebergTablePartitionSpec struct {
	SpecID types.Int64                  `tfsdk:"spec_id" json:"spec-id"`
	Fields []icebergTablePartitionField `tfsdk:"fields" json:"fields"`
}

func (s icebergTablePartitionSpec) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"spec_id": types.Int64Type,
		"fields": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: icebergTablePartitionField{}.AttrTypes(),
			},
		},
	}
}

func (s icebergTablePartitionSpec) MarshalJSON() ([]byte, error) {
	type Alias struct {
		SpecID *int                         `json:"spec-id,omitempty"`
		Fields []icebergTablePartitionField `json:"fields"`
	}
	var specID *int
	if !s.SpecID.IsNull() && !s.SpecID.IsUnknown() {
		val := int(s.SpecID.ValueInt64())
		specID = &val
	}

	return json.Marshal(&Alias{
		SpecID: specID,
		Fields: s.Fields,
	})
}

func (s *icebergTablePartitionSpec) UnmarshalJSON(b []byte) error {
	type Alias struct {
		SpecID int                          `json:"spec-id"`
		Fields []icebergTablePartitionField `json:"fields"`
	}
	var raw Alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.SpecID = types.Int64Value(int64(raw.SpecID))
	s.Fields = raw.Fields

	return nil
}

// ToIceberg converts the spec, first resolving source_names to field ids
// against schema (the planned table schema). The receiver is not modified.
func (s *icebergTablePartitionSpec) ToIceberg(schema *iceberg.Schema) (*iceberg.PartitionSpec, error) {
	resolved := *s
	resolved.Fields = slices.Clone(s.Fields)
	if err := resolved.resolveSourceIDs(schema); err != nil {
		return nil, err
	}
	b, err := json.Marshal(resolved)
	if err != nil {
		return nil, err
	}
	var icebergSpec iceberg.PartitionSpec
	if err := json.Unmarshal(b, &icebergSpec); err != nil {
		return nil, err
	}

	return &icebergSpec, nil
}

// FromIceberg converts icebergSpec, naming each field's source columns from
// schema (the table's current schema).
func (s *icebergTablePartitionSpec) FromIceberg(icebergSpec iceberg.PartitionSpec, schema *iceberg.Schema) error {
	s.SpecID = types.Int64Value(int64(icebergSpec.ID()))
	s.Fields = make([]icebergTablePartitionField, 0, icebergSpec.NumFields())
	for _, field := range icebergSpec.Fields() {
		s.Fields = append(s.Fields, icebergTablePartitionField{
			SourceIDs:   sourceIDsList(field.SourceIDs),
			SourceNames: sourceNamesList(schema, field.SourceIDs),
			FieldID:     types.Int64Value(int64(field.FieldID)),
			Name:        field.Name,
			Transform:   field.Transform.String(),
		})
	}

	return nil
}

// resolveSourceIDs fills source_ids from source_names for fields that name
// their source columns instead of giving ids, and fails for a field with
// neither. Known source_ids are kept: config never sets both (ExactlyOneOf),
// and an update plan marks the unset one unknown, so known source_ids here
// always come from config.
func (s *icebergTablePartitionSpec) resolveSourceIDs(schema *iceberg.Schema) error {
	for i := range s.Fields {
		f := &s.Fields[i]
		if isKnown(f.SourceIDs) {
			continue
		}
		if !isKnown(f.SourceNames) {
			return fmt.Errorf("partition field %q has neither source_ids nor source_names", f.Name)
		}
		names := f.SourceNames.Elements()
		ids := make([]attr.Value, len(names))
		for j, elem := range names {
			name, ok := elem.(types.String)
			if !ok || !isKnown(name) {
				return fmt.Errorf("partition field %q: source_names must not contain null or unknown values", f.Name)
			}
			id, err := resolveSourceColumn(schema, name.ValueString())
			if err != nil {
				return fmt.Errorf("partition field %q: %w", f.Name, err)
			}
			ids[j] = types.Int64Value(int64(id))
		}
		f.SourceIDs = types.ListValueMust(types.Int64Type, ids)
	}

	return nil
}

// icebergTablePartitionField mirrors an Iceberg partition field. SourceNames
// exists only in Terraform: it is resolved to SourceIDs before conversion and
// never sent to the catalog.
type icebergTablePartitionField struct {
	SourceIDs   types.List  `tfsdk:"source_ids" json:"source-ids"`
	SourceNames types.List  `tfsdk:"source_names" json:"-"`
	FieldID     types.Int64 `tfsdk:"field_id" json:"field-id,omitempty"`
	Name        string      `tfsdk:"name" json:"name"`
	Transform   string      `tfsdk:"transform" json:"transform"`
}

// sourceIDValues returns the field's source ids, failing if they are still
// unresolved.
func (f icebergTablePartitionField) sourceIDValues() ([]int64, error) {
	if !isKnown(f.SourceIDs) {
		return nil, fmt.Errorf("partition field %q has no resolved source_ids", f.Name)
	}
	elems := f.SourceIDs.Elements()
	ids := make([]int64, len(elems))
	for i, elem := range elems {
		id, ok := elem.(types.Int64)
		if !ok || !isKnown(id) {
			return nil, fmt.Errorf("partition field %q: source_ids must not contain null or unknown values", f.Name)
		}
		ids[i] = id.ValueInt64()
	}

	return ids, nil
}

func (f icebergTablePartitionField) MarshalJSON() ([]byte, error) {
	type Alias struct {
		SourceIDs []int64 `json:"source-ids"`
		FieldID   int64   `json:"field-id,omitempty"`
		Name      string  `json:"name"`
		Transform string  `json:"transform"`
	}
	sourceIDs, err := f.sourceIDValues()
	if err != nil {
		return nil, err
	}
	var fieldID int64
	if !f.FieldID.IsNull() && !f.FieldID.IsUnknown() {
		fieldID = f.FieldID.ValueInt64()
	}

	return json.Marshal(&Alias{
		SourceIDs: sourceIDs,
		FieldID:   fieldID,
		Name:      f.Name,
		Transform: f.Transform,
	})
}

func (f *icebergTablePartitionField) UnmarshalJSON(b []byte) error {
	type Alias struct {
		SourceIDs []int  `json:"source-ids"`
		FieldID   int64  `json:"field-id"`
		Name      string `json:"name"`
		Transform string `json:"transform"`
	}
	var raw Alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	f.SourceIDs = sourceIDsList(raw.SourceIDs)
	f.SourceNames = types.ListNull(types.StringType)
	f.FieldID = types.Int64Value(raw.FieldID)
	f.Name = raw.Name
	f.Transform = raw.Transform

	return nil
}

func (icebergTablePartitionField) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_ids":   types.ListType{ElemType: types.Int64Type},
		"source_names": types.ListType{ElemType: types.StringType},
		"field_id":     types.Int64Type,
		"name":         types.StringType,
		"transform":    types.StringType,
	}
}

type icebergTableSortOrder struct {
	OrderID types.Int64             `tfsdk:"order_id" json:"order-id"`
	Fields  []icebergTableSortField `tfsdk:"fields" json:"fields"`
}

func (s icebergTableSortOrder) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"order_id": types.Int64Type,
		"fields": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: icebergTableSortField{}.AttrTypes(),
			},
		},
	}
}

// MarshalJSON always emits "order-id" because iceberg-go requires it. An unset
// ID defaults to the unsorted order ID when there are no fields and to the
// initial sort order ID otherwise.
func (s icebergTableSortOrder) MarshalJSON() ([]byte, error) {
	type Alias struct {
		OrderID int                     `json:"order-id"`
		Fields  []icebergTableSortField `json:"fields"`
	}
	orderID := table.InitialSortOrderID
	switch {
	case !s.OrderID.IsNull() && !s.OrderID.IsUnknown():
		orderID = int(s.OrderID.ValueInt64())
	case len(s.Fields) == 0:
		orderID = table.UnsortedSortOrderID
	}
	fields := s.Fields
	if fields == nil {
		fields = []icebergTableSortField{}
	}

	return json.Marshal(&Alias{
		OrderID: orderID,
		Fields:  fields,
	})
}

func (s *icebergTableSortOrder) UnmarshalJSON(b []byte) error {
	type Alias struct {
		OrderID int                     `json:"order-id"`
		Fields  []icebergTableSortField `json:"fields"`
	}
	var raw Alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.OrderID = types.Int64Value(int64(raw.OrderID))
	s.Fields = raw.Fields

	return nil
}

// ToIceberg converts the sort order, first resolving source_name to a field id
// against schema (the planned table schema). The receiver is not modified.
func (s *icebergTableSortOrder) ToIceberg(schema *iceberg.Schema) (table.SortOrder, error) {
	resolved := *s
	resolved.Fields = slices.Clone(s.Fields)
	if err := resolved.resolveSourceIDs(schema); err != nil {
		return table.SortOrder{}, err
	}
	b, err := json.Marshal(resolved)
	if err != nil {
		return table.SortOrder{}, err
	}
	var icebergOrder table.SortOrder
	if err := json.Unmarshal(b, &icebergOrder); err != nil {
		return table.SortOrder{}, err
	}

	return icebergOrder, nil
}

// FromIceberg converts icebergOrder, naming each field's source column from
// schema (the table's current schema).
func (s *icebergTableSortOrder) FromIceberg(icebergOrder table.SortOrder, schema *iceberg.Schema) error {
	s.OrderID = types.Int64Value(int64(icebergOrder.OrderID()))
	b, err := json.Marshal(icebergOrder)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return err
	}
	for i := range s.Fields {
		f := &s.Fields[i]
		if name, ok := sourceColumnName(schema, int(f.SourceID.ValueInt64())); ok {
			f.SourceName = types.StringValue(name)
		}
	}

	return nil
}

// resolveSourceIDs fills source_id from source_name for fields that name
// their source column instead of giving an id, and fails for a field with
// neither. A known source_id is kept, for the same reason as in the partition
// spec's resolveSourceIDs.
func (s *icebergTableSortOrder) resolveSourceIDs(schema *iceberg.Schema) error {
	for i := range s.Fields {
		f := &s.Fields[i]
		if isKnown(f.SourceID) {
			continue
		}
		if !isKnown(f.SourceName) {
			return fmt.Errorf("sort_order.fields[%d] has neither source_id nor source_name", i)
		}
		id, err := resolveSourceColumn(schema, f.SourceName.ValueString())
		if err != nil {
			return fmt.Errorf("sort_order.fields[%d]: %w", i, err)
		}
		f.SourceID = types.Int64Value(int64(id))
	}

	return nil
}

// icebergTableSortField mirrors an Iceberg sort field. SourceName exists only
// in Terraform: it is resolved to SourceID before conversion and never sent
// to the catalog.
type icebergTableSortField struct {
	SourceID   types.Int64  `tfsdk:"source_id" json:"source-id"`
	SourceName types.String `tfsdk:"source_name" json:"-"`
	Transform  string       `tfsdk:"transform" json:"transform"`
	Direction  string       `tfsdk:"direction" json:"direction"`
	NullOrder  string       `tfsdk:"null_order" json:"null-order"`
}

func (icebergTableSortField) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_id":   types.Int64Type,
		"source_name": types.StringType,
		"transform":   types.StringType,
		"direction":   types.StringType,
		"null_order":  types.StringType,
	}
}

func (f icebergTableSortField) MarshalJSON() ([]byte, error) {
	if !isKnown(f.SourceID) {
		return nil, errors.New("sort field has no resolved source_id")
	}

	return json.Marshal(struct {
		SourceID  int64  `json:"source-id"`
		Transform string `json:"transform"`
		Direction string `json:"direction"`
		NullOrder string `json:"null-order"`
	}{
		SourceID:  f.SourceID.ValueInt64(),
		Transform: f.Transform,
		Direction: f.Direction,
		NullOrder: f.NullOrder,
	})
}

func (f *icebergTableSortField) UnmarshalJSON(b []byte) error {
	var raw struct {
		SourceID  int64  `json:"source-id"`
		Transform string `json:"transform"`
		Direction string `json:"direction"`
		NullOrder string `json:"null-order"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	f.SourceID = types.Int64Value(raw.SourceID)
	f.SourceName = types.StringNull()
	f.Transform = raw.Transform
	f.Direction = raw.Direction
	f.NullOrder = raw.NullOrder

	return nil
}

// resolveSourceColumn returns the id of the column named name. The lookup is
// case-sensitive and requires the canonical full name (e.g. "points.element.x",
// not iceberg-go's "points.x" alias): state stores the canonical name read
// back from the table, so any other spelling would not match the config.
func resolveSourceColumn(schema *iceberg.Schema, name string) (int, error) {
	if schema == nil {
		return 0, fmt.Errorf("cannot resolve source column %q without a table schema", name)
	}
	field, ok := schema.FindFieldByName(name)
	if !ok {
		return 0, fmt.Errorf("source column %q not found in the configured schema", name)
	}
	if canonical, ok := schema.FindColumnName(field.ID); !ok || canonical != name {
		return 0, fmt.Errorf("source column %q must be referenced by its full name %q", name, canonical)
	}

	return field.ID, nil
}

// sourceColumnName returns the canonical name of the column with the given
// id, or false if schema has no such column.
func sourceColumnName(schema *iceberg.Schema, id int) (string, bool) {
	if schema == nil {
		return "", false
	}

	return schema.FindColumnName(id)
}

func sourceIDsList(ids []int) types.List {
	elems := make([]attr.Value, len(ids))
	for i, id := range ids {
		elems[i] = types.Int64Value(int64(id))
	}

	return types.ListValueMust(types.Int64Type, elems)
}

// sourceNamesList names each id from schema; it is null if any id is missing.
func sourceNamesList(schema *iceberg.Schema, ids []int) types.List {
	elems := make([]attr.Value, len(ids))
	for i, id := range ids {
		name, ok := sourceColumnName(schema, id)
		if !ok {
			return types.ListNull(types.StringType)
		}
		elems[i] = types.StringValue(name)
	}

	return types.ListValueMust(types.StringType, elems)
}

// isKnown reports whether v is neither null nor unknown.
func isKnown(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

type icebergTableSchemaField struct {
	ID               types.Int64                              `tfsdk:"id" json:"id"`
	Name             string                                   `tfsdk:"name" json:"name"`
	Type             string                                   `tfsdk:"type" json:"-"`
	Required         bool                                     `tfsdk:"required" json:"required"`
	Doc              *string                                  `tfsdk:"doc" json:"doc,omitempty"`
	ListProperties   *icebergTableSchemaFieldListProperties   `tfsdk:"list_properties" json:"-"`
	MapProperties    *icebergTableSchemaFieldMapProperties    `tfsdk:"map_properties" json:"-"`
	StructProperties *icebergTableSchemaFieldStructProperties `tfsdk:"struct_properties" json:"-"`
}

func (icebergTableSchemaField) AttrTypes(depth int) map[string]attr.Type {
	res := map[string]attr.Type{
		"id":              types.Int64Type,
		"name":            types.StringType,
		"type":            types.StringType,
		"required":        types.BoolType,
		"doc":             types.StringType,
		"list_properties": types.ObjectType{AttrTypes: icebergTableSchemaFieldListProperties{}.AttrTypes()},
		"map_properties":  types.ObjectType{AttrTypes: icebergTableSchemaFieldMapProperties{}.AttrTypes()},
	}

	if depth > 0 {
		res["struct_properties"] = types.ObjectType{AttrTypes: icebergTableSchemaFieldStructProperties{}.AttrTypes(depth - 1)}
	} else {
		// At max depth, we still need the attribute defined but it won't have fields
		res["struct_properties"] = types.ObjectType{AttrTypes: map[string]attr.Type{
			"fields": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{}}},
		}}
	}

	return res
}

func (f icebergTableSchemaField) MarshalJSON() ([]byte, error) {
	return marshalFieldJSON(f.ID, f.Name, f.Type, f.Required, f.Doc, f.ListProperties, f.MapProperties, f.StructProperties)
}

func (f *icebergTableSchemaField) UnmarshalJSON(b []byte) error {
	return unmarshalFieldJSON(b, &f.ID, &f.Name, &f.Type, &f.Required, &f.Doc, &f.ListProperties, &f.MapProperties, &f.StructProperties)
}

type icebergTableSchemaFieldListProperties struct {
	ID              types.Int64 `tfsdk:"element_id" json:"element-id"`
	Type            string      `tfsdk:"element_type" json:"element"`
	ElementRequired bool        `tfsdk:"element_required" json:"element-required"`
}

func (icebergTableSchemaFieldListProperties) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"element_id":       types.Int64Type,
		"element_type":     types.StringType,
		"element_required": types.BoolType,
	}
}

func (p icebergTableSchemaFieldListProperties) MarshalJSON() ([]byte, error) {
	var elementID int64
	if !p.ID.IsNull() && !p.ID.IsUnknown() {
		elementID = p.ID.ValueInt64()
	}

	return json.Marshal(struct {
		Type            string `json:"type"`
		ElementID       int64  `json:"element-id"`
		ElementType     string `json:"element"`
		ElementRequired bool   `json:"element-required"`
	}{
		Type:            "list",
		ElementID:       elementID,
		ElementType:     p.Type,
		ElementRequired: p.ElementRequired,
	})
}

func (p *icebergTableSchemaFieldListProperties) UnmarshalJSON(b []byte) error {
	var raw struct {
		ElementID       int64  `json:"element-id"`
		ElementType     string `json:"element"`
		ElementRequired bool   `json:"element-required"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.ID = types.Int64Value(raw.ElementID)
	p.Type = raw.ElementType
	p.ElementRequired = raw.ElementRequired

	return nil
}

type icebergTableSchemaFieldMapProperties struct {
	KeyID         types.Int64 `tfsdk:"key_id" json:"key-id"`
	KeyType       string      `tfsdk:"key_type" json:"key"`
	ValueID       types.Int64 `tfsdk:"value_id" json:"value-id"`
	ValueType     string      `tfsdk:"value_type" json:"value"`
	ValueRequired bool        `tfsdk:"value_required" json:"value-required"`
}

func (icebergTableSchemaFieldMapProperties) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key_id":         types.Int64Type,
		"key_type":       types.StringType,
		"value_id":       types.Int64Type,
		"value_type":     types.StringType,
		"value_required": types.BoolType,
	}
}

func (p icebergTableSchemaFieldMapProperties) MarshalJSON() ([]byte, error) {
	var keyID, valueID int64
	if !p.KeyID.IsNull() && !p.KeyID.IsUnknown() {
		keyID = p.KeyID.ValueInt64()
	}
	if !p.ValueID.IsNull() && !p.ValueID.IsUnknown() {
		valueID = p.ValueID.ValueInt64()
	}

	return json.Marshal(struct {
		Type          string `json:"type"`
		KeyID         int64  `json:"key-id"`
		KeyType       string `json:"key"`
		ValueID       int64  `json:"value-id"`
		ValueType     string `json:"value"`
		ValueRequired bool   `json:"value-required"`
	}{
		Type:          "map",
		KeyID:         keyID,
		KeyType:       p.KeyType,
		ValueID:       valueID,
		ValueType:     p.ValueType,
		ValueRequired: p.ValueRequired,
	})
}

func (p *icebergTableSchemaFieldMapProperties) UnmarshalJSON(b []byte) error {
	var raw struct {
		KeyID         int64  `json:"key-id"`
		KeyType       string `json:"key"`
		ValueID       int64  `json:"value-id"`
		ValueType     string `json:"value"`
		ValueRequired bool   `json:"value-required"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.KeyID = types.Int64Value(raw.KeyID)
	p.KeyType = raw.KeyType
	p.ValueID = types.Int64Value(raw.ValueID)
	p.ValueType = raw.ValueType
	p.ValueRequired = raw.ValueRequired

	return nil
}

type icebergTableSchemaFieldStructProperties struct {
	Fields []icebergTableSchemaField `tfsdk:"fields" json:"fields"`
}

func (icebergTableSchemaFieldStructProperties) AttrTypes(depth int) map[string]attr.Type {
	return map[string]attr.Type{
		"fields": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: icebergTableSchemaField{}.AttrTypes(depth),
			},
		},
	}
}

func (s icebergTableSchemaFieldStructProperties) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string                    `json:"type"`
		Fields []icebergTableSchemaField `json:"fields"`
	}{
		Type:   "struct",
		Fields: s.Fields,
	})
}

func (s *icebergTableSchemaFieldStructProperties) UnmarshalJSON(b []byte) error {
	var raw struct {
		Fields []icebergTableSchemaField `json:"fields"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.Fields = raw.Fields

	return nil
}

// Helpers for shared logic

func marshalFieldJSON(id types.Int64, name, typeStr string, required bool, doc *string, listProps, mapProps, structProps interface{}) ([]byte, error) {
	type Field struct {
		ID       int64       `json:"id"`
		Name     string      `json:"name"`
		Type     interface{} `json:"type"`
		Required bool        `json:"required"`
		Doc      *string     `json:"doc,omitempty"`
	}

	var idVal int64
	if !id.IsNull() && !id.IsUnknown() {
		idVal = id.ValueInt64()
	}

	f := Field{
		ID:       idVal,
		Name:     name,
		Required: required,
		Doc:      doc,
	}

	switch typeStr {
	case "list":
		f.Type = listProps
	case "map":
		f.Type = mapProps
	case "struct":
		f.Type = structProps
	default:
		f.Type = typeStr
	}

	return json.Marshal(f)
}

func unmarshalFieldJSON(b []byte, id *types.Int64, name, typeStr *string, required *bool, doc **string, listProps, mapProps, structProps interface{}) error {
	var raw struct {
		ID       int64           `json:"id"`
		Name     string          `json:"name"`
		Type     json.RawMessage `json:"type"`
		Required bool            `json:"required"`
		Doc      *string         `json:"doc"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*id = types.Int64Value(raw.ID)
	*name = raw.Name
	*required = raw.Required
	*doc = raw.Doc

	if len(raw.Type) > 0 && raw.Type[0] == '"' {
		var s string
		if err := json.Unmarshal(raw.Type, &s); err != nil {
			return err
		}
		// Server + user may use different amounts of whitespace - decimal(10, 2)
		*typeStr = strings.ReplaceAll(s, " ", "")
	} else {
		var typeObj struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw.Type, &typeObj); err != nil {
			return err
		}
		*typeStr = typeObj.Type
		switch typeObj.Type {
		case "list":
			return json.Unmarshal(raw.Type, listProps)
		case "map":
			return json.Unmarshal(raw.Type, mapProps)
		case "struct":
			return json.Unmarshal(raw.Type, structProps)
		}
	}

	return nil
}
