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
	_ resource.Resource                = &polarisPrincipalRoleResource{}
	_ resource.ResourceWithImportState = &polarisPrincipalRoleResource{}
)

func NewPolarisPrincipalRoleResource() resource.Resource {
	return &polarisPrincipalRoleResource{}
}

type polarisPrincipalRoleResource struct {
	provider         *icebergProvider
	managementClient *polarisManagementClient
}

type polarisPrincipalRoleResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Federated  types.Bool   `tfsdk:"federated"`
	Properties types.Map    `tfsdk:"properties"`
}

func (r *polarisPrincipalRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_principal_role"
}

func (r *polarisPrincipalRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource for managing Polaris principal roles.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the principal role.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"federated": schema.BoolAttribute{
				Description: "Whether the role is managed by an external identity provider. Immutable after creation.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					requiresReplaceIfChanged(),
				},
			},
			"properties": schema.MapAttribute{
				Description: "Arbitrary metadata properties for the principal role.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *polarisPrincipalRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	provider, ok := req.ProviderData.(*icebergProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *icebergProvider, got a different type. Please report this issue to the provider developers.",
		)

		return
	}
	r.provider = provider
}

func (r *polarisPrincipalRoleResource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
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

func (r *polarisPrincipalRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	props := mapToStringMap(ctx, data.Properties)

	createReq := polarisCreatePrincipalRoleRequest{
		PrincipalRole: polarisPrincipalRole{
			Name:       data.Name.ValueString(),
			Federated:  data.Federated.ValueBool(),
			Properties: props,
		},
	}

	tflog.Info(ctx, "Creating Polaris principal role", map[string]any{"name": data.Name.ValueString()})

	created, err := r.managementClient.CreatePrincipalRole(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create principal role", err.Error())

		return
	}

	data.ID = types.StringValue(created.Name)
	data.Name = types.StringValue(created.Name)
	data.Federated = types.BoolValue(created.Federated)
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

func (r *polarisPrincipalRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.managementClient.GetPrincipalRole(ctx, data.Name.ValueString())
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read principal role", err.Error())

		return
	}

	data.ID = types.StringValue(role.Name)
	data.Name = types.StringValue(role.Name)
	data.Federated = types.BoolValue(role.Federated)
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

func (r *polarisPrincipalRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan, state polarisPrincipalRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()

	current, err := r.managementClient.GetPrincipalRole(ctx, name)
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read principal role for update", err.Error())

		return
	}

	props := mapToStringMap(ctx, plan.Properties)

	updateReq := polarisUpdatePrincipalRoleRequest{
		CurrentEntityVersion: current.EntityVersion,
		Properties:           props,
	}

	updated, err := retryOnConflict(func() (*polarisPrincipalRole, error) {
		return r.managementClient.UpdatePrincipalRole(ctx, name, updateReq)
	}, func() (*polarisPrincipalRole, error) {
		current, err := r.managementClient.GetPrincipalRole(ctx, name)
		if err != nil {
			return nil, err
		}
		updateReq.CurrentEntityVersion = current.EntityVersion

		return r.managementClient.UpdatePrincipalRole(ctx, name, updateReq)
	})
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to update principal role", err.Error())

		return
	}

	state.Federated = types.BoolValue(updated.Federated)
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

func (r *polarisPrincipalRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Deleting Polaris principal role", map[string]any{"name": data.Name.ValueString()})

	err := r.managementClient.DeletePrincipalRole(ctx, data.Name.ValueString())
	if err != nil && !isPolarisNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete principal role", err.Error())
	}
}

func (r *polarisPrincipalRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
