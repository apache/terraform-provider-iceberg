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
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &polarisPrincipalResource{}
	_ resource.ResourceWithImportState = &polarisPrincipalResource{}
)

func NewPolarisPrincipalResource() resource.Resource {
	return &polarisPrincipalResource{}
}

type polarisPrincipalResource struct {
	provider *icebergProvider
	// managementClient calls Polaris Management API only; catalog access uses iceberg-go REST via catalog_uri.
	managementClient *polarisManagementClient
}

type polarisPrincipalResourceModel struct {
	ID                         types.String `tfsdk:"id"`
	Name                       types.String `tfsdk:"name"`
	Properties                 types.Map    `tfsdk:"properties"`
	CredentialRotationRequired types.Bool   `tfsdk:"credential_rotation_required"`
	ClientID                   types.String `tfsdk:"client_id"`
	ClientSecret               types.String `tfsdk:"client_secret"`
}

func (r *polarisPrincipalResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_principal"
}

func (r *polarisPrincipalResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource for managing Polaris principals and their client credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the Polaris principal.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.MapAttribute{
				Description: "Arbitrary metadata properties for the principal.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"credential_rotation_required": schema.BoolAttribute{
				Description: "If true, the initial credentials can only be used to call rotateCredentials.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"client_id": schema.StringAttribute{
				Description: "The client ID associated with this principal. Computed after create.",
				Computed:    true,
				Sensitive:   true,
			},
			"client_secret": schema.StringAttribute{
				Description: "The client secret associated with this principal. Polaris only allows setting/resetting via resetCredentials after create; this provider stores the secret after create and preserves it on update.",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *polarisPrincipalResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*icebergProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *icebergProvider, got a different type: %T. Please report this issue to the provider developers.",
		)

		return
	}
	r.provider = provider
}

func (r *polarisPrincipalResource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
	if r.managementClient != nil {
		return
	}
	if r.provider == nil {
		diags.AddError(
			"Provider not configured",
			"The provider hasn't been configured before this operation")

		return
	}
	client, err := r.provider.newPolarisManagementClient()
	if err != nil {
		diags.AddError("Failed to create Polaris management API client", err.Error())

		return
	}
	r.managementClient = client
}

func (r *polarisPrincipalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalResourceModel

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()

	props := make(map[string]string)
	if !data.Properties.IsNull() && !data.Properties.IsUnknown() {
		diags = data.Properties.ElementsAs(ctx, &props, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var rotationRequired *bool
	if !data.CredentialRotationRequired.IsNull() && !data.CredentialRotationRequired.IsUnknown() {
		v := data.CredentialRotationRequired.ValueBool()
		rotationRequired = &v
	}

	reqBody := polarisCreatePrincipalRequest{
		Principal: polarisPrincipal{
			Name:       name,
			Properties: props,
		},
		CredentialRotationRequired: rotationRequired,
	}

	tflog.Info(ctx, "Creating Polaris principal", map[string]any{"name": name})

	created, err := r.managementClient.CreatePrincipal(ctx, reqBody)
	if err != nil {
		resp.Diagnostics.AddError("failed to create principal", err.Error())

		return
	}

	data.ID = types.StringValue(created.Principal.Name)
	data.Name = types.StringValue(created.Principal.Name)

	if len(created.Principal.Properties) > 0 {
		propsVal, diags := types.MapValueFrom(ctx, types.StringType, created.Principal.Properties)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Properties = propsVal
	} else {
		data.Properties = types.MapNull(types.StringType)
	}

	data.ClientID = types.StringValue(created.Credentials.ClientID)
	data.ClientSecret = types.StringValue(created.Credentials.ClientSecret)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *polarisPrincipalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()

	tflog.Info(ctx, "Reading Polaris principal", map[string]any{"name": name})

	principal, err := r.managementClient.GetPrincipal(ctx, name)
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read Polaris principal", err.Error())

		return
	}

	// Credential rotation flag is not returned by the API on GET.
	// Preserve from state if available, otherwise default to false.
	if data.CredentialRotationRequired.IsNull() || data.CredentialRotationRequired.IsUnknown() {
		data.CredentialRotationRequired = types.BoolValue(false)
	}

	// Update properties; credentials are not returned on GET so keep from state.
	if len(principal.Properties) > 0 {
		propsVal, diags := types.MapValueFrom(ctx, types.StringType, principal.Properties)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Properties = propsVal
	} else {
		data.Properties = types.MapNull(types.StringType)
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *polarisPrincipalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan, state polarisPrincipalResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()

	// Fetch current entity version from API so we don't require users to track it; Terraform manages it internally.
	current, err := r.managementClient.GetPrincipal(ctx, name)
	if err != nil {
		var nf *polarisNotFoundError
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read Polaris principal for update", err.Error())

		return
	}

	props := make(map[string]string)
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		diags = plan.Properties.ElementsAs(ctx, &props, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	updateReq := polarisUpdatePrincipalRequest{
		CurrentEntityVersion: current.EntityVersion,
		Properties:           props,
	}

	tflog.Info(ctx, "Updating Polaris principal", map[string]any{"name": name})

	updated, err := r.managementClient.UpdatePrincipal(ctx, name, updateReq)
	if err != nil {
		var nf *polarisNotFoundError
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to update Polaris principal", err.Error())

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

	// Credential rotation flag not returned by API on update — preserve from plan
	val := false
	if !plan.CredentialRotationRequired.IsNull() && !plan.CredentialRotationRequired.IsUnknown() {
		val = plan.CredentialRotationRequired.ValueBool()
	}
	state.CredentialRotationRequired = types.BoolValue(val)

	// Polaris doesn't return credentials on update, and setting the secret requires
	// a dedicated resetCredentials call after create. Preserve everything from state.
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *polarisPrincipalResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()

	tflog.Info(ctx, "Deleting Polaris principal", map[string]any{"name": name})

	err := r.managementClient.DeletePrincipal(ctx, name)
	if err != nil && !isPolarisNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete Polaris principal", err.Error())

		return
	}
}

func (r *polarisPrincipalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by principal name; set both id and name.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
