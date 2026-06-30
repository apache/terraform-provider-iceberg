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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &polarisPrincipalRoleDataSource{}

func NewPolarisPrincipalRoleDataSource() datasource.DataSource {
	return &polarisPrincipalRoleDataSource{}
}

type polarisPrincipalRoleDataSource struct {
	provider         *icebergProvider
	managementClient *polarisManagementClient
}

type polarisPrincipalRoleDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Federated  types.Bool   `tfsdk:"federated"`
	Properties types.Map    `tfsdk:"properties"`
}

func (d *polarisPrincipalRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_principal_role"
}

func (d *polarisPrincipalRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Polaris principal role by name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the principal role.",
				Required:    true,
			},
			"federated": schema.BoolAttribute{
				Description: "Whether the role is managed by an external identity provider.",
				Computed:    true,
			},
			"properties": schema.MapAttribute{
				Description: "Arbitrary metadata properties for the principal role.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *polarisPrincipalRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	provider, ok := req.ProviderData.(*icebergProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"Expected *icebergProvider, got a different type.",
		)

		return
	}
	d.provider = provider
}

func (d *polarisPrincipalRoleDataSource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
	if d.managementClient != nil {
		return
	}
	if d.provider == nil {
		diags.AddError("Provider not configured", "The provider hasn't been configured before this operation")

		return
	}
	client, err := d.provider.newPolarisManagementClient()
	if err != nil {
		diags.AddError("Failed to create Polaris management API client", err.Error())

		return
	}
	d.managementClient = client
}

func (d *polarisPrincipalRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	d.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisPrincipalRoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := d.managementClient.GetPrincipalRole(ctx, data.Name.ValueString())
	if err != nil {
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
