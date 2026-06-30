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
	_ resource.Resource                = &polarisCatalogRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &polarisCatalogRoleAssignmentResource{}
)

func NewPolarisCatalogRoleAssignmentResource() resource.Resource {
	return &polarisCatalogRoleAssignmentResource{}
}

type polarisCatalogRoleAssignmentResource struct {
	provider         *icebergProvider
	managementClient *polarisManagementClient
}

type polarisCatalogRoleAssignmentResourceModel struct {
	ID                types.String `tfsdk:"id"`
	PrincipalRoleName types.String `tfsdk:"principal_role_name"`
	CatalogName       types.String `tfsdk:"catalog_name"`
	CatalogRoleName   types.String `tfsdk:"catalog_role_name"`
}

func (r *polarisCatalogRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_catalog_role_assignment"
}

func (r *polarisCatalogRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a catalog role to a principal role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"principal_role_name": schema.StringAttribute{
				Description: "The principal role receiving the assignment.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog_name": schema.StringAttribute{
				Description: "The catalog containing the role to assign.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog_role_name": schema.StringAttribute{
				Description: "The catalog role to assign to the principal role.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *polarisCatalogRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *polarisCatalogRoleAssignmentResource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
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

func (r *polarisCatalogRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisCatalogRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Assigning catalog role to principal role", map[string]any{
		"principal_role": data.PrincipalRoleName.ValueString(),
		"catalog":        data.CatalogName.ValueString(),
		"catalog_role":   data.CatalogRoleName.ValueString(),
	})

	err := r.managementClient.AssignCatalogRoleToPrincipalRole(
		ctx,
		data.PrincipalRoleName.ValueString(),
		data.CatalogName.ValueString(),
		data.CatalogRoleName.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to assign catalog role to principal role", err.Error())

		return
	}

	data.ID = types.StringValue(
		data.PrincipalRoleName.ValueString() + "/" +
			data.CatalogName.ValueString() + "/" +
			data.CatalogRoleName.ValueString(),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisCatalogRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisCatalogRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	catalogRoles, err := r.managementClient.ListCatalogRoleAssignments(
		ctx,
		data.PrincipalRoleName.ValueString(),
		data.CatalogName.ValueString(),
	)
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to list catalog role assignments", err.Error())

		return
	}

	found := false
	for _, role := range catalogRoles.Roles {
		if role.Name == data.CatalogRoleName.ValueString() {
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

func (r *polarisCatalogRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Change attributes to force recreation.")
}

func (r *polarisCatalogRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisCatalogRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Revoking catalog role from principal role", map[string]any{
		"principal_role": data.PrincipalRoleName.ValueString(),
		"catalog":        data.CatalogName.ValueString(),
		"catalog_role":   data.CatalogRoleName.ValueString(),
	})

	err := r.managementClient.RevokeCatalogRoleFromPrincipalRole(
		ctx,
		data.PrincipalRoleName.ValueString(),
		data.CatalogName.ValueString(),
		data.CatalogRoleName.ValueString(),
	)
	if err != nil && !isPolarisNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to revoke catalog role from principal role", err.Error())
	}
}

func (r *polarisCatalogRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: {principal_role_name}/{catalog_name}/{catalog_role_name}")

		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_role_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role_name"), parts[2])...)
}
