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
	"fmt"
	"net/http"

	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/catalog/rest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ provider.Provider                   = &icebergProvider{}
	_ provider.ProviderWithValidateConfig = &icebergProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New() func() provider.Provider {
	return func() provider.Provider {
		return &icebergProvider{}
	}
}

// icebergProvider is the provider implementation.
type icebergProvider struct {
	catalogURI           string
	catalogType          string
	token                string
	warehouse            string
	headers              map[string]string
	sigv4Enabled         bool
	sigv4Region          string
	sigv4SigningName     string
	sigv4AccessKeyID     string
	sigv4SecretAccessKey string
	sigv4SessionToken    string
}

// icebergProviderModel maps provider schema data to a Go type.
type icebergProviderModel struct {
	CatalogURI           types.String `tfsdk:"catalog_uri"`
	Type                 types.String `tfsdk:"type"`
	Token                types.String `tfsdk:"token"`
	Warehouse            types.String `tfsdk:"warehouse"`
	Headers              types.Map    `tfsdk:"headers"`
	SigV4Enabled         types.Bool   `tfsdk:"sigv4_enabled"`
	SigV4Region          types.String `tfsdk:"sigv4_region"`
	SigV4SigningName     types.String `tfsdk:"sigv4_signing_name"`
	SigV4AccessKeyID     types.String `tfsdk:"sigv4_access_key_id"`
	SigV4SecretAccessKey types.String `tfsdk:"sigv4_secret_access_key"`
	SigV4SessionToken    types.String `tfsdk:"sigv4_session_token"`
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
			"sigv4_enabled": schema.BoolAttribute{
				Description: "Enable AWS SigV4 request signing for the REST catalog. Required by catalogs that authenticate with SigV4, such as AWS Glue.",
				Optional:    true,
			},
			"sigv4_region": schema.StringAttribute{
				Description: "Signing region for SigV4. When omitted, the region from the AWS environment (`AWS_REGION`, shared config) is used.",
				Optional:    true,
			},
			"sigv4_signing_name": schema.StringAttribute{
				Description: "Signing service name for SigV4 (the credential-scope service). Defaults to `execute-api`. Use `glue` for AWS Glue.",
				Optional:    true,
			},
			"sigv4_access_key_id": schema.StringAttribute{
				Description: "Access key ID for SigV4 signing. When omitted, the standard AWS credential chain (environment, shared config, instance role) is used.",
				Optional:    true,
				Sensitive:   true,
			},
			"sigv4_secret_access_key": schema.StringAttribute{
				Description: "Secret access key for SigV4 signing. Must be paired with `sigv4_access_key_id`.",
				Optional:    true,
				Sensitive:   true,
			},
			"sigv4_session_token": schema.StringAttribute{
				Description: "Optional session token for temporary (STS) SigV4 credentials.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

// ValidateConfig rejects incoherent SigV4 configuration before Configure runs.
func (p *icebergProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	var data icebergProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateSigV4Config(data, &resp.Diagnostics)
}

// validateSigV4Config reports SigV4 misconfigurations. Checks that need an
// unknown value are skipped; Terraform validates again at apply with concrete values.
func validateSigV4Config(data icebergProviderModel, diags *diag.Diagnostics) {
	// presence reports whether v is a known, non-empty string, and whether it
	// is resolved (not unknown).
	presence := func(v types.String) (set, known bool) {
		if v.IsUnknown() {
			return false, false
		}

		return !v.IsNull() && v.ValueString() != "", true
	}

	accessKeySet, accessKeyKnown := presence(data.SigV4AccessKeyID)
	secretKeySet, secretKeyKnown := presence(data.SigV4SecretAccessKey)
	sessionTokenSet, sessionTokenKnown := presence(data.SigV4SessionToken)
	regionSet, regionKnown := presence(data.SigV4Region)
	signingNameSet, signingNameKnown := presence(data.SigV4SigningName)

	enabled := !data.SigV4Enabled.IsNull() && !data.SigV4Enabled.IsUnknown() && data.SigV4Enabled.ValueBool()
	enabledKnown := !data.SigV4Enabled.IsUnknown()

	if accessKeyKnown && secretKeyKnown && accessKeySet != secretKeySet {
		diags.AddError(
			"Incomplete SigV4 credentials",
			"sigv4_access_key_id and sigv4_secret_access_key must be set together.",
		)
	}

	if sessionTokenKnown && accessKeyKnown && secretKeyKnown &&
		sessionTokenSet && !(accessKeySet && secretKeySet) {
		diags.AddError(
			"Incomplete SigV4 credentials",
			"sigv4_session_token requires sigv4_access_key_id and sigv4_secret_access_key.",
		)
	}

	optionSet := (accessKeyKnown && accessKeySet) || (secretKeyKnown && secretKeySet) ||
		(sessionTokenKnown && sessionTokenSet) || (regionKnown && regionSet) ||
		(signingNameKnown && signingNameSet)
	if enabledKnown && !enabled && optionSet {
		diags.AddWarning(
			"SigV4 options ignored",
			"sigv4_* options are set but sigv4_enabled is not true; requests will not be signed.",
		)
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

	if !data.SigV4Enabled.IsNull() && !data.SigV4Enabled.IsUnknown() {
		p.sigv4Enabled = data.SigV4Enabled.ValueBool()
	}
	if !data.SigV4Region.IsNull() && !data.SigV4Region.IsUnknown() {
		p.sigv4Region = data.SigV4Region.ValueString()
	}
	if !data.SigV4SigningName.IsNull() && !data.SigV4SigningName.IsUnknown() {
		p.sigv4SigningName = data.SigV4SigningName.ValueString()
	}
	if !data.SigV4AccessKeyID.IsNull() && !data.SigV4AccessKeyID.IsUnknown() {
		p.sigv4AccessKeyID = data.SigV4AccessKeyID.ValueString()
	}
	if !data.SigV4SecretAccessKey.IsNull() && !data.SigV4SecretAccessKey.IsUnknown() {
		p.sigv4SecretAccessKey = data.SigV4SecretAccessKey.ValueString()
	}
	if !data.SigV4SessionToken.IsNull() && !data.SigV4SessionToken.IsUnknown() {
		p.sigv4SessionToken = data.SigV4SessionToken.ValueString()
	}

	resp.DataSourceData = p
	resp.ResourceData = p
}

func (p *icebergProvider) NewCatalog(ctx context.Context) (catalog.Catalog, error) {
	opts := make([]rest.Option, 0)
	switch {
	case p.token != "" && p.sigv4Enabled:
		// SigV4 owns Authorization; the bearer token travels, signed, as
		// Original-Authorization, as the Java client does.
		opts = append(opts, rest.WithHeaders(map[string]string{"Original-Authorization": "Bearer " + p.token}))
	case p.token != "":
		opts = append(opts, rest.WithOAuthToken(p.token))
	}

	if p.warehouse != "" {
		opts = append(opts, rest.WithWarehouseLocation(p.warehouse))
	}

	opts = append(opts, rest.WithCustomTransport(&headerRoundTripper{headers: p.headers}))

	if p.sigv4Enabled {
		region := p.sigv4Region
		if p.sigv4AccessKeyID != "" && p.sigv4SecretAccessKey != "" {
			if region == "" {
				cfg, err := config.LoadDefaultConfig(ctx)
				if err != nil {
					return nil, fmt.Errorf("resolve sigv4_region from the AWS environment: %w", err)
				}
				if cfg.Region == "" {
					return nil, errors.New("sigv4_region is required: no region is configured in the AWS environment")
				}
				region = cfg.Region
			}
			creds := credentials.NewStaticCredentialsProvider(
				p.sigv4AccessKeyID, p.sigv4SecretAccessKey, p.sigv4SessionToken)
			opts = append(opts, rest.WithAwsConfig(aws.Config{
				Region:      region,
				Credentials: aws.NewCredentialsCache(creds),
			}))
		}
		opts = append(opts, rest.WithSigV4RegionSvc(region, p.sigv4SigningName))
	}

	return rest.NewCatalog(ctx, p.catalogType, p.catalogURI, opts...)
}

type headerRoundTripper struct {
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Add(k, v)
	}

	return http.DefaultTransport.RoundTrip(req)
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
