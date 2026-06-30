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
	_ resource.Resource                = &polarisCatalogRoleResource{}
	_ resource.ResourceWithImportState = &polarisCatalogRoleResource{}
)

func NewPolarisCatalogRoleResource() resource.Resource {
	return &polarisCatalogRoleResource{}
}

type polarisCatalogRoleResource struct {
	provider         *icebergProvider
	managementClient *polarisManagementClient
}

type polarisCatalogRoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	CatalogName types.String `tfsdk:"catalog_name"`
	Name        types.String `tfsdk:"name"`
	Properties  types.Map    `tfsdk:"properties"`
}

func (r *polarisCatalogRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_catalog_role"
}

func (r *polarisCatalogRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource for managing Polaris catalog roles.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"catalog_name": schema.StringAttribute{
				Description: "The catalog this role belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the catalog role.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.MapAttribute{
				Description: "Arbitrary metadata properties for the catalog role.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *polarisCatalogRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *polarisCatalogRoleResource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
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

func (r *polarisCatalogRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisCatalogRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	props := mapToStringMap(ctx, data.Properties)

	createReq := polarisCreateCatalogRoleRequest{
		CatalogRole: polarisCatalogRole{
			Name:       data.Name.ValueString(),
			Properties: props,
		},
	}

	tflog.Info(ctx, "Creating Polaris catalog role", map[string]any{
		"catalog": data.CatalogName.ValueString(),
		"name":    data.Name.ValueString(),
	})

	created, err := r.managementClient.CreateCatalogRole(ctx, data.CatalogName.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create catalog role", err.Error())

		return
	}

	data.ID = types.StringValue(data.CatalogName.ValueString() + "/" + created.Name)
	data.Name = types.StringValue(created.Name)
	if len(created.Properties) > 0 {
		propsVal, diags := types.MapValueFrom(ctx, types.StringType, created.Properties)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Properties = propsVal
	} else {
		data.Properties = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisCatalogRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisCatalogRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.managementClient.GetCatalogRole(ctx, data.CatalogName.ValueString(), data.Name.ValueString())
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read catalog role", err.Error())

		return
	}

	data.Name = types.StringValue(role.Name)
	if len(role.Properties) > 0 {
		propsVal, diags := types.MapValueFrom(ctx, types.StringType, role.Properties)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Properties = propsVal
	} else {
		data.Properties = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisCatalogRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan, state polarisCatalogRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	catalogName := state.CatalogName.ValueString()
	roleName := state.Name.ValueString()

	current, err := r.managementClient.GetCatalogRole(ctx, catalogName, roleName)
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read catalog role for update", err.Error())

		return
	}

	props := mapToStringMap(ctx, plan.Properties)

	updateReq := polarisUpdateCatalogRoleRequest{
		CurrentEntityVersion: current.EntityVersion,
		Properties:           props,
	}

	updated, err := retryOnConflict(func() (*polarisCatalogRole, error) {
		return r.managementClient.UpdateCatalogRole(ctx, catalogName, roleName, updateReq)
	}, func() (*polarisCatalogRole, error) {
		current, err := r.managementClient.GetCatalogRole(ctx, catalogName, roleName)
		if err != nil {
			return nil, err
		}
		updateReq.CurrentEntityVersion = current.EntityVersion

		return r.managementClient.UpdateCatalogRole(ctx, catalogName, roleName, updateReq)
	})
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to update catalog role", err.Error())

		return
	}

	if len(updated.Properties) > 0 {
		propsVal, diags := types.MapValueFrom(ctx, types.StringType, updated.Properties)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Properties = propsVal
	} else {
		state.Properties = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *polarisCatalogRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisCatalogRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Deleting Polaris catalog role", map[string]any{
		"catalog": data.CatalogName.ValueString(),
		"name":    data.Name.ValueString(),
	})

	err := r.managementClient.DeleteCatalogRole(ctx, data.CatalogName.ValueString(), data.Name.ValueString())
	if err != nil && !isPolarisNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete catalog role", err.Error())
	}
}

func (r *polarisCatalogRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: {catalog_name}/{role_name}")

		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
}
