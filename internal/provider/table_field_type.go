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
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.StringTypable                    = fieldTypeStringType{}
	_ basetypes.StringValuableWithSemanticEquals = fieldTypeString{}
)

// fieldTypeStringType is the attribute type for Iceberg type names such as
// "decimal(10, 2)". The catalog and the configuration may disagree on
// whitespace, so values compare equal when they only differ by spaces.
type fieldTypeStringType struct {
	basetypes.StringType
}

func (t fieldTypeStringType) Equal(o attr.Type) bool {
	other, ok := o.(fieldTypeStringType)

	return ok && t.StringType.Equal(other.StringType)
}

func (fieldTypeStringType) String() string {
	return "fieldTypeStringType"
}

func (fieldTypeStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return fieldTypeString{StringValue: in}, nil
}

func (t fieldTypeStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	v, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	s, ok := v.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", v)
	}

	return fieldTypeString{StringValue: s}, nil
}

func (fieldTypeStringType) ValueType(context.Context) attr.Value {
	return fieldTypeString{}
}

type fieldTypeString struct {
	basetypes.StringValue
}

func (v fieldTypeString) Equal(o attr.Value) bool {
	other, ok := o.(fieldTypeString)

	return ok && v.StringValue.Equal(other.StringValue)
}

func (fieldTypeString) Type(context.Context) attr.Type {
	return fieldTypeStringType{}
}

func (v fieldTypeString) StringSemanticEquals(_ context.Context, o basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	other, ok := o.(fieldTypeString)
	if !ok {
		diags.AddError("Semantic Equality Check Error", fmt.Sprintf("expected fieldTypeString, got %T", o))

		return false, diags
	}

	return stripSpaces(v.ValueString()) == stripSpaces(other.ValueString()), diags
}

func stripSpaces(s string) string {
	return strings.ReplaceAll(s, " ", "")
}
