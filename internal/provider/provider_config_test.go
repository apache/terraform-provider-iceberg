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
	"maps"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaTypes returns the Terraform types of the provider config, auth and auth.sigv4.
func schemaTypes(t *testing.T) (config, auth, sigv4 tftypes.Object) {
	t.Helper()
	ctx := context.Background()

	var schemaResp provider.SchemaResponse
	New()().Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	config, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	require.True(t, ok)
	auth, ok = config.AttributeTypes["auth"].(tftypes.Object)
	require.True(t, ok)
	sigv4, ok = auth.AttributeTypes["sigv4"].(tftypes.Object)
	require.True(t, ok)

	return config, auth, sigv4
}

// object builds a value of typ with the given attributes set and the rest null.
func object(typ tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	vals := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, attrType := range typ.AttributeTypes {
		vals[name] = tftypes.NewValue(attrType, nil)
	}
	maps.Copy(vals, set)

	return tftypes.NewValue(typ, vals)
}

// providerConfigWith builds a provider configuration with catalog_uri and the given attributes set.
func providerConfigWith(t *testing.T, set map[string]tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	configType, _, _ := schemaTypes(t)
	vals := map[string]tftypes.Value{"catalog_uri": str("http://localhost:8181")}
	maps.Copy(vals, set)
	config, err := tfprotov6.NewDynamicValue(configType, object(configType, vals))
	require.NoError(t, err)

	return &config
}

// authProviderConfig builds a provider configuration with catalog_uri and the given auth value.
func authProviderConfig(t *testing.T, auth tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()

	return providerConfigWith(t, map[string]tftypes.Value{"auth": auth})
}

// oauth2ObjectType returns the Terraform type of auth.oauth2.
func oauth2ObjectType(t *testing.T, auth tftypes.Object) tftypes.Object {
	t.Helper()
	oauth2, ok := auth.AttributeTypes["oauth2"].(tftypes.Object)
	require.True(t, ok)

	return oauth2
}

// sigv4ProviderConfig builds a provider configuration that sets the given auth.sigv4 attributes.
func sigv4ProviderConfig(t *testing.T, sigv4 map[string]tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	_, authType, sigv4Type := schemaTypes(t)

	return authProviderConfig(t, object(authType, map[string]tftypes.Value{"sigv4": object(sigv4Type, sigv4)}))
}

func errorDiagnostics(diags []*tfprotov6.Diagnostic) []string {
	var errs []string
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			errs = append(errs, d.Summary+": "+d.Detail)
		}
	}

	return errs
}

func str(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }

func TestConfigureReadsSigV4Settings(t *testing.T) {
	p := &icebergProvider{}
	server, err := providerserver.NewProtocol6WithError(p)()
	require.NoError(t, err)

	resp, err := server.ConfigureProvider(context.Background(), &tfprotov6.ConfigureProviderRequest{
		Config: sigv4ProviderConfig(t, map[string]tftypes.Value{
			"region":            str("us-east-1"),
			"signing_name":      str("glue"),
			"access_key_id":     str("AKID"),
			"secret_access_key": str("SECRET"),
			"session_token":     str("TOKEN"),
		}),
	})
	require.NoError(t, err)
	require.Empty(t, errorDiagnostics(resp.Diagnostics))

	assert.Equal(t, &sigv4Config{
		region:          "us-east-1",
		signingName:     "glue",
		accessKeyID:     "AKID",
		secretAccessKey: "SECRET",
		sessionToken:    "TOKEN",
	}, p.sigv4)
}

func TestConfigureClearsSigV4WhenRemoved(t *testing.T) {
	_, authType, _ := schemaTypes(t)
	p := &icebergProvider{}
	server, err := providerserver.NewProtocol6WithError(p)()
	require.NoError(t, err)

	for _, config := range []*tfprotov6.DynamicValue{
		sigv4ProviderConfig(t, map[string]tftypes.Value{"region": str("us-east-1")}),
		authProviderConfig(t, tftypes.NewValue(authType, nil)),
	} {
		resp, err := server.ConfigureProvider(context.Background(), &tfprotov6.ConfigureProviderRequest{Config: config})
		require.NoError(t, err)
		require.Empty(t, errorDiagnostics(resp.Diagnostics))
	}

	assert.Nil(t, p.sigv4, "a provider configured again without auth must stop signing")
}

func TestValidateSigV4RequiresCompleteKeyPair(t *testing.T) {
	for name, tc := range map[string]struct {
		sigv4 map[string]tftypes.Value
		want  []string
	}{
		"access key only":    {map[string]tftypes.Value{"access_key_id": str("AKID")}, []string{`"auth.sigv4.secret_access_key" must be specified`}},
		"secret key only":    {map[string]tftypes.Value{"secret_access_key": str("SECRET")}, []string{`"auth.sigv4.access_key_id" must be specified`}},
		"session token only": {map[string]tftypes.Value{"session_token": str("TOKEN")}, []string{`"auth.sigv4.access_key_id" must be specified`, `"auth.sigv4.secret_access_key" must be specified`}},
		"complete keys":      {map[string]tftypes.Value{"access_key_id": str("AKID"), "secret_access_key": str("SECRET"), "session_token": str("TOKEN")}, nil},
		"empty keys": {map[string]tftypes.Value{"access_key_id": str(""), "secret_access_key": str("")}, []string{
			`auth.sigv4.access_key_id string length must be at least 1`, `auth.sigv4.secret_access_key string length must be at least 1`,
		}},
		"empty session token": {map[string]tftypes.Value{"access_key_id": str("AKID"), "secret_access_key": str("SECRET"), "session_token": str("")}, []string{
			`auth.sigv4.session_token string length must be at least 1`,
		}},
		"ambient chain": {map[string]tftypes.Value{"region": str("us-east-1")}, nil},
		"secret unknown at plan": {map[string]tftypes.Value{
			"access_key_id":     str("AKID"),
			"secret_access_key": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			server, err := providerserver.NewProtocol6WithError(New()())()
			require.NoError(t, err)
			resp, err := server.ValidateProviderConfig(context.Background(),
				&tfprotov6.ValidateProviderConfigRequest{Config: sigv4ProviderConfig(t, tc.sigv4)})
			require.NoError(t, err)

			errs := errorDiagnostics(resp.Diagnostics)
			if len(tc.want) == 0 {
				assert.Empty(t, errs)

				return
			}
			for _, want := range tc.want {
				assert.Contains(t, strings.Join(errs, "\n"), want)
			}
		})
	}
}

func TestNewCatalogRefusesAuthUnknownAtPlan(t *testing.T) {
	isolateAWSEnv(t)
	_, authType, sigv4Type := schemaTypes(t)
	oauth2Type := oauth2ObjectType(t, authType)
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	withSigV4 := func(set map[string]tftypes.Value) tftypes.Value {
		return object(authType, map[string]tftypes.Value{"sigv4": object(sigv4Type, set)})
	}

	for name, auth := range map[string]tftypes.Value{
		"keys":         withSigV4(map[string]tftypes.Value{"region": str("us-east-1"), "access_key_id": unknown, "secret_access_key": unknown}),
		"region":       withSigV4(map[string]tftypes.Value{"region": unknown}),
		"signing name": withSigV4(map[string]tftypes.Value{"region": str("us-east-1"), "signing_name": unknown}),
		"sigv4":        object(authType, map[string]tftypes.Value{"sigv4": tftypes.NewValue(sigv4Type, tftypes.UnknownValue)}),
		"auth":         tftypes.NewValue(authType, tftypes.UnknownValue),
		"oauth2 credential": object(authType, map[string]tftypes.Value{
			"oauth2": object(oauth2Type, map[string]tftypes.Value{"credential": unknown}),
		}),
		"oauth2 server uri": object(authType, map[string]tftypes.Value{
			"oauth2": object(oauth2Type, map[string]tftypes.Value{"credential": str("client:secret"), "server_uri": unknown}),
		}),
	} {
		t.Run(name, func(t *testing.T) {
			p := &icebergProvider{}
			server, err := providerserver.NewProtocol6WithError(p)()
			require.NoError(t, err)
			resp, err := server.ConfigureProvider(context.Background(),
				&tfprotov6.ConfigureProviderRequest{Config: authProviderConfig(t, auth)})
			require.NoError(t, err)
			require.Empty(t, errorDiagnostics(resp.Diagnostics))

			headers, err := configHeaders(t, p)
			require.ErrorContains(t, err, "not known until apply")

			assert.Nil(t, headers, "nothing may be sent until auth is known")
		})
	}
}

func TestValidateOAuth2Settings(t *testing.T) {
	_, authType, sigv4Type := schemaTypes(t)
	oauth2Type := oauth2ObjectType(t, authType)
	withOAuth2 := func(set map[string]tftypes.Value) tftypes.Value {
		return object(authType, map[string]tftypes.Value{"oauth2": object(oauth2Type, set)})
	}
	credential := map[string]tftypes.Value{"credential": str("client:secret")}

	for name, tc := range map[string]struct {
		config map[string]tftypes.Value
		want   string
	}{
		"credential only": {map[string]tftypes.Value{"auth": withOAuth2(credential)}, ""},
		"conflicts with token": {map[string]tftypes.Value{
			"token": str("static"),
			"auth":  withOAuth2(credential),
		}, "Invalid Attribute Combination"},
		"conflicts with sigv4": {map[string]tftypes.Value{"auth": object(authType, map[string]tftypes.Value{
			"oauth2": object(oauth2Type, credential),
			"sigv4":  object(sigv4Type, map[string]tftypes.Value{"region": str("us-east-1")}),
		})}, "Invalid Attribute Combination"},
		"empty credential": {map[string]tftypes.Value{
			"auth": withOAuth2(map[string]tftypes.Value{"credential": str("")}),
		}, "auth.oauth2.credential string length must be at least 1"},
		"relative server_uri": {map[string]tftypes.Value{
			"auth": withOAuth2(map[string]tftypes.Value{"credential": str("client:secret"), "server_uri": str("/oauth/tokens")}),
		}, "Invalid OAuth2 Server URI"},
	} {
		t.Run(name, func(t *testing.T) {
			server, err := providerserver.NewProtocol6WithError(New()())()
			require.NoError(t, err)
			config := providerConfigWith(t, tc.config)

			validateResp, err := server.ValidateProviderConfig(context.Background(),
				&tfprotov6.ValidateProviderConfigRequest{Config: config})
			require.NoError(t, err)
			configureResp, err := server.ConfigureProvider(context.Background(),
				&tfprotov6.ConfigureProviderRequest{Config: config})
			require.NoError(t, err)

			errs := append(errorDiagnostics(validateResp.Diagnostics), errorDiagnostics(configureResp.Diagnostics)...)
			if tc.want == "" {
				assert.Empty(t, errs)

				return
			}
			assert.Contains(t, strings.Join(errs, "\n"), tc.want)
		})
	}
}
