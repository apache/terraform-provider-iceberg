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

func (s *icebergTablePartitionSpec) ToIceberg() (*iceberg.PartitionSpec, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var icebergSpec iceberg.PartitionSpec
	if err := json.Unmarshal(b, &icebergSpec); err != nil {
		return nil, err
	}

	return &icebergSpec, nil
}

func (s *icebergTablePartitionSpec) FromIceberg(icebergSpec iceberg.PartitionSpec) error {
	s.SpecID = types.Int64Value(int64(icebergSpec.ID()))
	s.Fields = make([]icebergTablePartitionField, 0, icebergSpec.NumFields())
	for _, field := range icebergSpec.Fields() {
		s.Fields = append(s.Fields, icebergTablePartitionField{
			SourceIDs: []int64{int64(field.SourceID())},
			FieldID:   types.Int64Value(int64(field.FieldID)),
			Name:      field.Name,
			Transform: field.Transform.String(),
		})
	}

	return nil
}

type icebergTablePartitionField struct {
	SourceIDs []int64     `tfsdk:"source_ids" json:"source-ids"`
	FieldID   types.Int64 `tfsdk:"field_id" json:"field-id,omitempty"`
	Name      string      `tfsdk:"name" json:"name"`
	Transform string      `tfsdk:"transform" json:"transform"`
}

func (f icebergTablePartitionField) MarshalJSON() ([]byte, error) {
	type Alias struct {
		SourceIDs []int64 `json:"source-ids"`
		FieldID   int64   `json:"field-id,omitempty"`
		Name      string  `json:"name"`
		Transform string  `json:"transform"`
	}
	var fieldID int64
	if !f.FieldID.IsNull() && !f.FieldID.IsUnknown() {
		fieldID = f.FieldID.ValueInt64()
	}

	return json.Marshal(&Alias{
		SourceIDs: f.SourceIDs,
		FieldID:   fieldID,
		Name:      f.Name,
		Transform: f.Transform,
	})
}

func (f *icebergTablePartitionField) UnmarshalJSON(b []byte) error {
	type Alias struct {
		SourceIDs []int64 `json:"source-ids"`
		FieldID   int64   `json:"field-id"`
		Name      string  `json:"name"`
		Transform string  `json:"transform"`
	}
	var raw Alias
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	f.SourceIDs = raw.SourceIDs
	f.FieldID = types.Int64Value(raw.FieldID)
	f.Name = raw.Name
	f.Transform = raw.Transform

	return nil
}

func (icebergTablePartitionField) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_ids": types.ListType{ElemType: types.Int64Type},
		"field_id":   types.Int64Type,
		"name":       types.StringType,
		"transform":  types.StringType,
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

func (s icebergTableSortOrder) MarshalJSON() ([]byte, error) {
	type Alias struct {
		OrderID *int                    `json:"order-id,omitempty"`
		Fields  []icebergTableSortField `json:"fields"`
	}
	var orderID *int
	if !s.OrderID.IsNull() && !s.OrderID.IsUnknown() {
		val := int(s.OrderID.ValueInt64())
		orderID = &val
	}

	return json.Marshal(&Alias{
		OrderID: orderID,
		Fields:  s.Fields,
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

func (s *icebergTableSortOrder) ToIceberg() (table.SortOrder, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return table.SortOrder{}, err
	}
	var icebergOrder table.SortOrder
	if err := json.Unmarshal(b, &icebergOrder); err != nil {
		return table.SortOrder{}, err
	}

	return icebergOrder, nil
}

func (s *icebergTableSortOrder) FromIceberg(icebergOrder table.SortOrder) error {
	s.OrderID = types.Int64Value(int64(icebergOrder.OrderID()))
	b, err := json.Marshal(icebergOrder)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, s)
}

type icebergTableSortField struct {
	SourceID  int64  `tfsdk:"source_id" json:"source-id"`
	Transform string `tfsdk:"transform" json:"transform"`
	Direction string `tfsdk:"direction" json:"direction"`
	NullOrder string `tfsdk:"null_order" json:"null-order"`
}

func (icebergTableSortField) AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_id":  types.Int64Type,
		"transform":  types.StringType,
		"direction":  types.StringType,
		"null_order": types.StringType,
	}
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
