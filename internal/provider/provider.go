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
	"net/url"

	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/catalog/rest"
	"github.com/hashicorp/terraform-plugin-framework-validators/providervalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ provider.Provider                     = &icebergProvider{}
	_ provider.ProviderWithConfigValidators = &icebergProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New() func() provider.Provider {
	return func() provider.Provider {
		return &icebergProvider{}
	}
}

// icebergProvider is the provider implementation.
type icebergProvider struct {
	catalogURI  string
	catalogType string
	token       string
	warehouse   string
	headers     map[string]string
	oauth2      *oauth2Config
}

// oauth2Config holds the OAuth2 client credentials settings passed to iceberg-go.
type oauth2Config struct {
	credential string
	serverURI  *url.URL
	scope      string
	audience   string
	resource   string
}

// icebergProviderModel maps provider schema data to a Go type.
type icebergProviderModel struct {
	CatalogURI types.String `tfsdk:"catalog_uri"`
	Type       types.String `tfsdk:"type"`
	Token      types.String `tfsdk:"token"`
	Warehouse  types.String `tfsdk:"warehouse"`
	Headers    types.Map    `tfsdk:"headers"`
	Auth       types.Object `tfsdk:"auth"`
}

type icebergAuthModel struct {
	OAuth2 types.Object `tfsdk:"oauth2"`
}

type icebergOAuth2Model struct {
	Credential types.String `tfsdk:"credential"`
	ServerURI  types.String `tfsdk:"server_uri"`
	Scope      types.String `tfsdk:"scope"`
	Audience   types.String `tfsdk:"audience"`
	Resource   types.String `tfsdk:"resource"`
}

// Metadata returns the provider type name.
func (p *icebergProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "iceberg"
}

// Schema defines the provider-level schema for configuration data.
func (p *icebergProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use Terraform to interact with Iceberg REST Catalog instances.",
		Attributes: map[string]schema.Attribute{
			"catalog_uri": schema.StringAttribute{
				Description: "The URI of the Iceberg REST catalog.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of catalog. Use 'rest' for a plain REST catalog.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "The token to use for authentication.",
				Optional:    true,
				Sensitive:   true,
			},
			"warehouse": schema.StringAttribute{
				Description: "The warehouse to use for the Iceberg REST catalog. This will be passed as `warehouse` property in the catalog properties.",
				Optional:    true,
			},
			"headers": schema.MapAttribute{
				Description: "The headers to use for authentication.",
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
			},
			"auth": schema.SingleNestedAttribute{
				Description: "Authentication settings for the Iceberg REST catalog.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"oauth2": schema.SingleNestedAttribute{
						Description: "Authenticate with the OAuth2 client credentials flow. Tokens are fetched and refreshed automatically.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"credential": schema.StringAttribute{
								Description: "The client credential, formatted as `client_id:client_secret`. A value without a colon is used as the client secret with an empty client ID.",
								Required:    true,
								Sensitive:   true,
							},
							"server_uri": schema.StringAttribute{
								Description: "The OAuth2 token endpoint. Defaults to `{catalog_uri}/v1/oauth/tokens`.",
								Optional:    true,
							},
							"scope": schema.StringAttribute{
								Description: "The scope to request. Defaults to `catalog`.",
								Optional:    true,
							},
							"audience": schema.StringAttribute{
								Description: "The audience to request.",
								Optional:    true,
							},
							"resource": schema.StringAttribute{
								Description: "The resource to request.",
								Optional:    true,
							},
						},
					},
				},
			},
		},
	}
}

// ConfigValidators returns validators that apply across provider attributes.
func (p *icebergProvider) ConfigValidators(_ context.Context) []provider.ConfigValidator {
	return []provider.ConfigValidator{
		providervalidator.Conflicting(
			path.MatchRoot("token"),
			path.MatchRoot("auth").AtName("oauth2"),
		),
	}
}

// Configure prepares a Iceberg API client for data sources and resources.
func (p *icebergProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data icebergProviderModel

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.CatalogURI.IsUnknown() {
		return
	}

	p.catalogURI = data.CatalogURI.ValueString()

	// Determine catalog type: only "rest" is supported.
	catalogType := "rest"
	if !data.Type.IsNull() && !data.Type.IsUnknown() {
		catalogType = data.Type.ValueString()
	}

	if catalogType != "rest" {
		resp.Diagnostics.AddError(
			"Unsupported Catalog Type",
			"The provider supports 'rest'. Got: "+catalogType,
		)

		return
	}

	p.catalogType = "rest"

	if !data.Token.IsNull() && !data.Token.IsUnknown() {
		p.token = data.Token.ValueString()
	}

	if !data.Warehouse.IsNull() && !data.Warehouse.IsUnknown() {
		p.warehouse = data.Warehouse.ValueString()
	}

	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		headers := make(map[string]string)
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		p.headers = headers
	}

	if !data.Auth.IsNull() && !data.Auth.IsUnknown() {
		var auth icebergAuthModel
		resp.Diagnostics.Append(data.Auth.As(ctx, &auth, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		if !auth.OAuth2.IsNull() && !auth.OAuth2.IsUnknown() {
			p.oauth2 = configureOAuth2(ctx, auth.OAuth2, resp)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	resp.DataSourceData = p
	resp.ResourceData = p
}

func configureOAuth2(ctx context.Context, obj types.Object, resp *provider.ConfigureResponse) *oauth2Config {
	var m icebergOAuth2Model
	resp.Diagnostics.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return nil
	}

	cfg := &oauth2Config{
		credential: m.Credential.ValueString(),
		scope:      m.Scope.ValueString(),
		audience:   m.Audience.ValueString(),
		resource:   m.Resource.ValueString(),
	}

	if serverURI := m.ServerURI.ValueString(); serverURI != "" {
		u, err := url.Parse(serverURI)
		if err != nil || u.Scheme == "" || u.Host == "" {
			resp.Diagnostics.AddAttributeError(
				path.Root("auth").AtName("oauth2").AtName("server_uri"),
				"Invalid OAuth2 Server URI",
				"server_uri must be an absolute URL. Got: "+serverURI,
			)

			return nil
		}
		cfg.serverURI = u
	}

	return cfg
}

func (p *icebergProvider) NewCatalog(ctx context.Context) (catalog.Catalog, error) {
	opts := make([]rest.Option, 0)
	if p.token != "" {
		opts = append(opts, rest.WithOAuthToken(p.token))
	}

	if o := p.oauth2; o != nil {
		opts = append(opts, rest.WithCredential(o.credential))
		if o.serverURI != nil {
			opts = append(opts, rest.WithAuthURI(o.serverURI))
		}
		if o.scope != "" {
			opts = append(opts, rest.WithScope(o.scope))
		}
		if o.audience != "" {
			opts = append(opts, rest.WithAudience(o.audience))
		}
		if o.resource != "" {
			opts = append(opts, rest.WithResource(o.resource))
		}
	}

	if p.warehouse != "" {
		opts = append(opts, rest.WithWarehouseLocation(p.warehouse))
	}

	if len(p.headers) > 0 {
		opts = append(opts, rest.WithHeaders(p.headers))
	}

	return rest.NewCatalog(ctx, p.catalogType, p.catalogURI, opts...)
}

// DataSources defines the data sources implemented in the provider.
func (p *icebergProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNamespaceDataSource,
		NewTableDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *icebergProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
		NewTableResource,
	}
}
