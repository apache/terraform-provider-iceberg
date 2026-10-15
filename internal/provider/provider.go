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
	"net/url"

	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/catalog/rest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/processcreds"
	"github.com/hashicorp/terraform-plugin-framework-validators/providervalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	sigv4       *sigv4Config
	oauth2      *oauth2Config
	// authUnknown is set while planning when auth depends on values known only after apply.
	authUnknown bool
}

// oauth2Config holds the OAuth2 client credentials settings passed to iceberg-go.
type oauth2Config struct {
	credential string
	serverURI  *url.URL
	scope      string
	audience   string
	resource   string
}

// sigv4Config holds the settings for signing requests with AWS SigV4.
type sigv4Config struct {
	region          string
	signingName     string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
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
	SigV4  types.Object `tfsdk:"sigv4"`
	OAuth2 types.Object `tfsdk:"oauth2"`
}

type icebergSigV4Model struct {
	Region          types.String `tfsdk:"region"`
	SigningName     types.String `tfsdk:"signing_name"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	SessionToken    types.String `tfsdk:"session_token"`
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
				Description: "The headers to use for authentication. With `auth.oauth2`, an `Authorization` entry is not sent, and the other headers are also sent to the OAuth2 token endpoint.",
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
			},
			"auth": schema.SingleNestedAttribute{
				Description: "Authentication settings for the Iceberg REST catalog.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"sigv4": schema.SingleNestedAttribute{
						Description: "Sign requests with AWS Signature Version 4, as catalogs such as AWS Glue require. Setting this attribute enables signing.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"region": schema.StringAttribute{
								Description: "Signing region. When omitted, the region from the AWS environment (`AWS_REGION`, shared config) is used.",
								Optional:    true,
							},
							"signing_name": schema.StringAttribute{
								Description: "Signing service name (the credential-scope service). Defaults to `execute-api`. Use `glue` for AWS Glue.",
								Optional:    true,
							},
							"access_key_id": schema.StringAttribute{
								Description: "Access key ID. When omitted, the standard AWS credential chain (environment, shared config, instance role) is used.",
								Optional:    true,
								Sensitive:   true,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
									stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("secret_access_key")),
								},
							},
							"secret_access_key": schema.StringAttribute{
								Description: "Secret access key. Must be set together with `access_key_id`.",
								Optional:    true,
								Sensitive:   true,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
									stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("access_key_id")),
								},
							},
							"session_token": schema.StringAttribute{
								Description: "Session token for temporary (STS) credentials. Requires `access_key_id` and `secret_access_key`.",
								Optional:    true,
								Sensitive:   true,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
									stringvalidator.AlsoRequires(
										path.MatchRelative().AtParent().AtName("access_key_id"),
										path.MatchRelative().AtParent().AtName("secret_access_key"),
									),
								},
							},
						},
					},
					"oauth2": schema.SingleNestedAttribute{
						Description: "Authenticate with the OAuth2 client credentials flow. Tokens are fetched and refreshed automatically.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"credential": schema.StringAttribute{
								Description: "The client credential, formatted as `client_id:client_secret`. A value without a colon is used as the client secret with an empty client ID.",
								Required:    true,
								Sensitive:   true,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
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
		// SigV4 replaces the Authorization header that carries the OAuth2 token.
		providervalidator.Conflicting(
			path.MatchRoot("auth").AtName("sigv4"),
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

	p.sigv4 = nil
	p.oauth2 = nil
	p.authUnknown = false
	if !data.Auth.IsNull() {
		authValue, err := data.Auth.ToTerraformValue(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Invalid auth configuration", err.Error())

			return
		}

		if !authValue.IsFullyKnown() {
			// Guessing any part would authenticate with the wrong scope or identity.
			p.authUnknown = true
		} else {
			var auth icebergAuthModel
			resp.Diagnostics.Append(data.Auth.As(ctx, &auth, basetypes.ObjectAsOptions{})...)
			if resp.Diagnostics.HasError() {
				return
			}

			if !auth.SigV4.IsNull() {
				var m icebergSigV4Model
				resp.Diagnostics.Append(auth.SigV4.As(ctx, &m, basetypes.ObjectAsOptions{})...)
				if resp.Diagnostics.HasError() {
					return
				}

				p.sigv4 = &sigv4Config{
					region:          m.Region.ValueString(),
					signingName:     m.SigningName.ValueString(),
					accessKeyID:     m.AccessKeyID.ValueString(),
					secretAccessKey: m.SecretAccessKey.ValueString(),
					sessionToken:    m.SessionToken.ValueString(),
				}
			}

			if !auth.OAuth2.IsNull() {
				p.oauth2 = configureOAuth2(ctx, auth.OAuth2, resp)
				if resp.Diagnostics.HasError() {
					return
				}
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
	if p.authUnknown {
		return nil, errors.New("auth is not known until apply, so requests cannot be authenticated yet")
	}

	opts := make([]rest.Option, 0)
	if p.token != "" && p.sigv4 == nil {
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

	if p.sigv4 == nil {
		headers := p.headers
		if p.oauth2 != nil {
			headers = withoutAuthorization(headers)
		}
		if len(headers) > 0 {
			opts = append(opts, rest.WithHeaders(headers))
		}
		// Without a transport, iceberg-go builds one per catalog and never
		// closes its idle connections.
		opts = append(opts, rest.WithCustomTransport(http.DefaultTransport))
	} else {
		sigv4Opts, err := p.sigv4.options(ctx, p.token, p.headers)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sigv4Opts...)
	}

	return rest.NewCatalog(ctx, p.catalogType, p.catalogURI, opts...)
}

// withoutAuthorization drops the Authorization header, which the OAuth2 token
// replaces and which must not reach a separate token endpoint.
func withoutAuthorization(headers map[string]string) map[string]string {
	kept := make(map[string]string, len(headers))
	for name, value := range headers {
		if http.CanonicalHeaderKey(name) != "Authorization" {
			kept[name] = value
		}
	}

	return kept
}

// options returns the iceberg-go options that sign requests with SigV4.
// iceberg-go signs before a custom transport runs, so the configured headers
// go through it to be signed with the request. SigV4 owns Authorization, so an
// Authorization header or bearer token travels as Original-Authorization, as
// the Java client does.
func (s *sigv4Config) options(ctx context.Context, token string, headers map[string]string) ([]rest.Option, error) {
	cfg, err := s.awsConfig(ctx)
	if err != nil {
		return nil, err
	}

	signed := make(map[string]string, len(headers)+1)
	for name, value := range headers {
		if http.CanonicalHeaderKey(name) == "Authorization" {
			name = "Original-Authorization"
		}
		signed[name] = value
	}
	if token != "" {
		signed["Original-Authorization"] = "Bearer " + token
	}

	return []rest.Option{
		rest.WithHeaders(signed),
		rest.WithAwsConfig(cfg),
		rest.WithSigV4RegionSvc(cfg.Region, s.signingName),
		// Without a transport, iceberg-go builds one per catalog and never
		// closes its idle connections.
		rest.WithCustomTransport(http.DefaultTransport),
	}, nil
}

// awsConfig resolves the credentials and region to sign with. Static keys
// replace the AWS credential chain. The region falls back to the AWS
// environment, and credential providers that call STS get it as well.
func (s *sigv4Config) awsConfig(ctx context.Context) (aws.Config, error) {
	static := s.accessKeyID != "" && s.secretAccessKey != ""
	if !static && (s.accessKeyID != "" || s.secretAccessKey != "" || s.sessionToken != "") {
		// Never fall back to the AWS credential chain when keys were configured.
		return aws.Config{}, errors.New("auth.sigv4: access_key_id and secret_access_key must be set together, and session_token requires both")
	}

	var load []func(*config.LoadOptions) error
	if static {
		creds := credentials.NewStaticCredentialsProvider(s.accessKeyID, s.secretAccessKey, s.sessionToken)
		if s.region != "" {
			return aws.Config{Region: s.region, Credentials: creds}, nil
		}
		// Only the region comes from the AWS environment, not its credential chain.
		load = append(load, config.WithCredentialsProvider(creds))
	}
	if s.region != "" {
		load = append(load, config.WithRegion(s.region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, load...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load the AWS configuration for auth.sigv4: %w", err)
	}
	if cfg.Region == "" {
		return aws.Config{}, errors.New("auth.sigv4.region is required: no region is configured in the AWS environment")
	}
	cfg.Credentials = processOutputHidden{cfg.Credentials}

	return cfg, nil
}

// processOutputHidden keeps credential_process output out of errors: the AWS
// SDK quotes output it cannot parse, and that output carries the secrets.
type processOutputHidden struct{ aws.CredentialsProvider }

func (p processOutputHidden) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := p.CredentialsProvider.Retrieve(ctx)
	var processErr *processcreds.ProviderError
	if errors.As(err, &processErr) {
		return creds, errors.New("credential_process failed; its output is not shown because it can contain secrets")
	}

	return creds, err
}

// DataSources defines the data sources implemented in the provider.
func (p *icebergProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNamespaceDataSource,
		NewTableDataSource,
		NewTablesDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *icebergProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
		NewTableResource,
	}
}
