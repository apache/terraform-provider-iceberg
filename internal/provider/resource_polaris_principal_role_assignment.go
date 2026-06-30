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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &polarisPrincipalRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &polarisPrincipalRoleAssignmentResource{}
)

func NewPolarisPrincipalRoleAssignmentResource() resource.Resource {
	return &polarisPrincipalRoleAssignmentResource{}
}

type polarisPrincipalRoleAssignmentResource struct {
	provider         *icebergProvider
	managementClient *polarisManagementClient
}

type polarisPrincipalRoleAssignmentResourceModel struct {
	ID                types.String `tfsdk:"id"`
	PrincipalName     types.String `tfsdk:"principal_name"`
	PrincipalRoleName types.String `tfsdk:"principal_role_name"`
}

func (r *polarisPrincipalRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_principal_role_assignment"
}

func (r *polarisPrincipalRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a principal role to a Polaris principal.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"principal_name": schema.StringAttribute{
				Description: "The name of the principal to assign the role to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal_role_name": schema.StringAttribute{
				Description: "The name of the principal role to assign.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *polarisPrincipalRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	provider, ok := req.ProviderData.(*icebergProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *icebergProvider, got a different type.",
		)

		return
	}
	r.provider = provider
}

func (r *polarisPrincipalRoleAssignmentResource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
	if r.managementClient != nil {
		return
	}
	if r.provider == nil {
		diags.AddError("Provider not configured", "The provider hasn't been configured before this operation")

		return
	}
	client, err := r.provider.newPolarisManagementClient()
	if err != nil {
		diags.AddError("Failed to create Polaris management API client", err.Error())

		return
	}
	r.managementClient = client
}

func (r *polarisPrincipalRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Assigning principal role", map[string]any{
		"principal": data.PrincipalName.ValueString(),
		"role":      data.PrincipalRoleName.ValueString(),
	})

	err := r.managementClient.AssignPrincipalRole(ctx, data.PrincipalName.ValueString(), data.PrincipalRoleName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to assign principal role", err.Error())

		return
	}

	data.ID = types.StringValue(data.PrincipalName.ValueString() + "/" + data.PrincipalRoleName.ValueString())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisPrincipalRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	assignments, err := r.managementClient.ListPrincipalRoleAssignments(ctx, data.PrincipalName.ValueString())
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to list principal role assignments", err.Error())

		return
	}

	found := false
	for _, role := range assignments.Roles {
		if role.Name == data.PrincipalRoleName.ValueString() {
			found = true

			break
		}
	}
	if !found {
		resp.State.RemoveResource(ctx)

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisPrincipalRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No Update — attribute changes trigger destroy + recreate via RequiresReplace.
	// This should never be called, but handle gracefully.
	resp.Diagnostics.AddError("Update not supported", "Change principal_name or principal_role_name to force recreation.")
}

func (r *polarisPrincipalRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Revoking principal role", map[string]any{
		"principal": data.PrincipalName.ValueString(),
		"role":      data.PrincipalRoleName.ValueString(),
	})

	err := r.managementClient.RevokePrincipalRole(ctx, data.PrincipalName.ValueString(), data.PrincipalRoleName.ValueString())
	if err != nil && !isPolarisNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to revoke principal role", err.Error())
	}
}

func (r *polarisPrincipalRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: {principal_name}/{role_name}")

		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_role_name"), parts[1])...)
}
