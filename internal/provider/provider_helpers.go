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

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapToStringMap converts a types.Map to map[string]string.
func mapToStringMap(ctx context.Context, m types.Map) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	var out map[string]string
	diags := m.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil
	}

	return out
}

// requiresReplaceIfChanged returns a plan modifier that forces replacement
// when a bool attribute changes.
func requiresReplaceIfChanged() planmodifier.Bool {
	return &requiresReplaceBoolModifier{}
}

type requiresReplaceBoolModifier struct{}

func (m *requiresReplaceBoolModifier) Description(ctx context.Context) string {
	return "If the value of this attribute changes, Terraform will destroy and recreate the resource."
}

func (m *requiresReplaceBoolModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *requiresReplaceBoolModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() || req.ConfigValue.IsNull() {
		return
	}
	if req.StateValue.ValueBool() != req.ConfigValue.ValueBool() {
		resp.RequiresReplace = true
	}
}

// requiresReplaceIfListChanged returns a plan modifier that forces replacement
// when a list attribute changes.
func requiresReplaceIfListChanged() planmodifier.List {
	return &requiresReplaceListModifier{}
}

type requiresReplaceListModifier struct{}

func (m *requiresReplaceListModifier) Description(ctx context.Context) string {
	return "If the value of this attribute changes, Terraform will destroy and recreate the resource."
}

func (m *requiresReplaceListModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *requiresReplaceListModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.StateValue.IsNull() || req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.Equal(req.ConfigValue) {
		resp.RequiresReplace = true
	}
}

// retryOnConflict calls fn; if it returns a 409-like error, calls retryFn once.
// Used for optimistic locking retry.
func retryOnConflict[T any](fn func() (T, error), retryFn func() (T, error)) (T, error) {
	var zero T
	result, err := fn()
	if err != nil {
		if isConflictError(err) {
			result, err = retryFn()
			if err != nil {
				if isConflictError(err) {
					return zero, fmt.Errorf("concurrent modification: %w", err)
				}

				return zero, err
			}

			return result, nil
		}

		return zero, err
	}

	return result, nil
}

func isConflictError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status 409")
}
