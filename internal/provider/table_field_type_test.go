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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFieldTypeStringSemanticEquals(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "identical", a: "decimal(10,2)", b: "decimal(10,2)", want: true},
		{name: "space after comma", a: "decimal(10, 2)", b: "decimal(10,2)", want: true},
		{name: "spaces around both numbers", a: "decimal( 10 , 2 )", b: "decimal(10,2)", want: true},
		{name: "different precision", a: "decimal(10,2)", b: "decimal(12,2)", want: false},
		{name: "different type", a: "int", b: "long", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := fieldTypeString{StringValue: types.StringValue(tt.a)}
			b := fieldTypeString{StringValue: types.StringValue(tt.b)}

			got, diags := a.StringSemanticEquals(context.Background(), b)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tt.want {
				t.Errorf("StringSemanticEquals(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
