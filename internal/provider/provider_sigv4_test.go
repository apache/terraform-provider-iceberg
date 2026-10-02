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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configHeaders creates a catalog against a stub REST server and returns the
// headers of its /v1/config request, or nil when no request was sent.
func configHeaders(t *testing.T, p *icebergProvider) (http.Header, error) {
	t.Helper()
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/config" {
			headers = r.Header.Clone()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"defaults":{},"overrides":{}}`))
	}))
	defer srv.Close()

	p.catalogURI, p.catalogType = srv.URL, "rest"
	_, err := p.NewCatalog(context.Background())

	return headers, err
}

// isolateAWSEnv hides the host's AWS configuration and gives the AWS
// credential chain a known ambient key pair.
func isolateAWSEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "AWS_") {
			t.Setenv(name, "")
		}
	}
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/credentials")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "AMBIENTKEY")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "AMBIENTSECRET")
}

func TestNewCatalogSignsRequestsWithSigV4(t *testing.T) {
	headers, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{
		region:          "us-east-1",
		signingName:     "glue",
		accessKeyID:     "AKIDEXAMPLE",
		secretAccessKey: "SECRETEXAMPLE",
	}})
	require.NoError(t, err)

	assert.Regexp(t, `^AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/\d{8}/us-east-1/glue/aws4_request,`,
		headers.Get("Authorization"), "the credential scope carries the configured keys, region and signing name")
	assert.NotEmpty(t, headers.Get("x-amz-content-sha256"),
		"SigV4 requires the x-amz-content-sha256 header")
}

func TestNewCatalogDefaultsSigningNameToExecuteAPI(t *testing.T) {
	headers, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{
		region:          "us-east-1",
		accessKeyID:     "AKIDEXAMPLE",
		secretAccessKey: "SECRETEXAMPLE",
	}})
	require.NoError(t, err)

	assert.Contains(t, headers.Get("Authorization"), "/us-east-1/execute-api/aws4_request",
		"an omitted signing name must default to execute-api")
}

func TestNewCatalogSignsWithSessionToken(t *testing.T) {
	headers, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{
		region:          "us-east-1",
		accessKeyID:     "AKIDEXAMPLE",
		secretAccessKey: "SECRETEXAMPLE",
		sessionToken:    "SESSIONTOKEN",
	}})
	require.NoError(t, err)

	assert.Equal(t, "SESSIONTOKEN", headers.Get("X-Amz-Security-Token"),
		"temporary credentials must send X-Amz-Security-Token")
}

func TestNewCatalogWithoutSigV4SendsNoSignature(t *testing.T) {
	headers, err := configHeaders(t, &icebergProvider{})
	require.NoError(t, err)

	assert.Empty(t, headers.Get("Authorization"), "plain config must not send an Authorization header")
	assert.Empty(t, headers.Get("x-amz-content-sha256"), "plain config must not send x-amz-content-sha256")
}

func TestNewCatalogRelocatesBearerTokenUnderSigV4(t *testing.T) {
	headers, err := configHeaders(t, &icebergProvider{
		token: "bearer-example",
		sigv4: &sigv4Config{region: "us-east-1", accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE"},
	})
	require.NoError(t, err)

	assert.Contains(t, headers.Get("Authorization"), "AWS4-HMAC-SHA256",
		"SigV4 owns the Authorization header")
	assert.Equal(t, "Bearer bearer-example", headers.Get("Original-Authorization"),
		"the bearer token moves to Original-Authorization, as the Java client does")
	assert.Contains(t, headers.Get("Authorization"), "original-authorization",
		"the relocated header is part of the signed headers")
}

func TestNewCatalogRelocatesAuthorizationHeaderUnderSigV4(t *testing.T) {
	for _, name := range []string{"Authorization", "authorization"} {
		t.Run(name, func(t *testing.T) {
			headers, err := configHeaders(t, &icebergProvider{
				headers: map[string]string{name: "Bearer from-headers"},
				sigv4:   &sigv4Config{region: "us-east-1", accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE"},
			})
			require.NoError(t, err)

			assert.Equal(t, "Bearer from-headers", headers.Get("Original-Authorization"),
				"an Authorization header from headers is relocated, as the Java client does")
			assert.Contains(t, headers.Get("Authorization"), "AWS4-HMAC-SHA256")
		})
	}
}

func TestNewCatalogSignsAmbientCredentialsWithConfiguredRegion(t *testing.T) {
	isolateAWSEnv(t)
	t.Setenv("AWS_REGION", "eu-west-1")

	headers, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{region: "ap-southeast-2"}})
	require.NoError(t, err)

	assert.Regexp(t, `Credential=AMBIENTKEY/\d{8}/ap-southeast-2/execute-api/aws4_request`,
		headers.Get("Authorization"), "the configured region wins over the AWS environment")
}

func TestNewCatalogResolvesRegionFromEnvironment(t *testing.T) {
	isolateAWSEnv(t)
	t.Setenv("AWS_REGION", "eu-west-1")

	for name, tc := range map[string]struct {
		sigv4 *sigv4Config
		key   string
	}{
		"static credentials":  {&sigv4Config{accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE"}, "AKIDEXAMPLE"},
		"ambient credentials": {&sigv4Config{}, "AMBIENTKEY"},
	} {
		t.Run(name, func(t *testing.T) {
			headers, err := configHeaders(t, &icebergProvider{sigv4: tc.sigv4})
			require.NoError(t, err)

			assert.Regexp(t, `Credential=`+tc.key+`/\d{8}/eu-west-1/execute-api/aws4_request`,
				headers.Get("Authorization"), "an omitted region falls back to the AWS environment")
		})
	}
}

func TestNewCatalogRequiresARegion(t *testing.T) {
	isolateAWSEnv(t)

	for name, sigv4 := range map[string]*sigv4Config{
		"static credentials":  {accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE"},
		"ambient credentials": {},
	} {
		t.Run(name, func(t *testing.T) {
			headers, err := configHeaders(t, &icebergProvider{sigv4: sigv4})
			require.ErrorContains(t, err, "auth.sigv4.region is required")

			assert.Nil(t, headers, "no request may be signed with an empty region")
		})
	}
}

func TestNewCatalogRejectsPartialStaticCredentials(t *testing.T) {
	isolateAWSEnv(t)

	for name, sigv4 := range map[string]*sigv4Config{
		"access key only":    {region: "us-east-1", accessKeyID: "EXPLICITKEY"},
		"secret key only":    {region: "us-east-1", secretAccessKey: "EXPLICITSECRET"},
		"session token only": {region: "us-east-1", sessionToken: "SESSIONTOKEN"},
	} {
		t.Run(name, func(t *testing.T) {
			headers, err := configHeaders(t, &icebergProvider{sigv4: sigv4})
			require.ErrorContains(t, err, "must be set together")

			assert.Nil(t, headers, "partial keys must not fall back to the AWS credential chain")
		})
	}
}

func TestNewCatalogSignsCustomHeaders(t *testing.T) {
	headers, err := configHeaders(t, &icebergProvider{
		headers: map[string]string{"X-Iceberg-Access-Delegation": "remote-signing", "X-Custom": "custom"},
		sigv4:   &sigv4Config{region: "us-east-1", accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"remote-signing"}, headers.Values("X-Iceberg-Access-Delegation"),
		"a configured header replaces the client default instead of adding a second, unsigned value")
	assert.Contains(t, headers.Get("Authorization"), "x-custom",
		"configured headers are part of the signed headers")
}

func TestNewCatalogPassesConfiguredRegionToAmbientCredentials(t *testing.T) {
	isolateAWSEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	// An assume-role profile: its credential provider signs an STS request with
	// the region it was built with.
	var stsAuth string
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stsAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleResult><Credentials>` +
			`<AccessKeyId>STSKEY</AccessKeyId><SecretAccessKey>STSSECRET</SecretAccessKey><SessionToken>STSTOKEN</SessionToken>` +
			`<Expiration>2099-01-01T00:00:00Z</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`))
	}))
	defer sts.Close()
	configFile := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(configFile, []byte(`[profile assume]
role_arn = arn:aws:iam::123456789012:role/example
source_profile = base

[profile base]
aws_access_key_id = BASEKEY
aws_secret_access_key = BASESECRET
`), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_PROFILE", "assume")
	t.Setenv("AWS_ENDPOINT_URL_STS", sts.URL)

	headers, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{region: "ap-southeast-2"}})
	require.NoError(t, err)

	assert.Contains(t, stsAuth, "/ap-southeast-2/sts/aws4_request",
		"credential providers that call STS need the configured region too")
	assert.Regexp(t, `Credential=STSKEY/\d{8}/ap-southeast-2/execute-api/aws4_request`, headers.Get("Authorization"))
}

func TestNewCatalogResolvesRegionWithoutAmbientCredentialChain(t *testing.T) {
	isolateAWSEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	configFile := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(configFile, []byte(`[profile mfa]
region = eu-central-1
role_arn = arn:aws:iam::123456789012:role/example
source_profile = base
mfa_serial = arn:aws:iam::123456789012:mfa/example

[profile base]
aws_access_key_id = BASEKEY
aws_secret_access_key = BASESECRET
`), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_PROFILE", "mfa")

	headers, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE"}})
	require.NoError(t, err, "static keys must not need the profile's MFA credential chain")

	assert.Regexp(t, `Credential=AKIDEXAMPLE/\d{8}/eu-central-1/execute-api/aws4_request`, headers.Get("Authorization"),
		"the region still comes from the profile")
}

func TestNewCatalogHidesCredentialProcessOutput(t *testing.T) {
	isolateAWSEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	configFile := filepath.Join(t.TempDir(), "config")
	// The AWS SDK quotes output it cannot parse, and the output carries secrets.
	require.NoError(t, os.WriteFile(configFile, []byte("[profile process]\ncredential_process = echo SECRETVALUE\n"), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_PROFILE", "process")

	_, err := configHeaders(t, &icebergProvider{sigv4: &sigv4Config{region: "us-east-1"}})
	require.ErrorContains(t, err, "credential_process")

	assert.NotContains(t, err.Error(), "SECRETVALUE")
}

func TestNewCatalogSharesConnectionsUnderSigV4(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"defaults":{},"overrides":{}}`))
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	for range 5 {
		p := &icebergProvider{catalogURI: srv.URL, catalogType: "rest", sigv4: &sigv4Config{
			region: "us-east-1", accessKeyID: "AKIDEXAMPLE", secretAccessKey: "SECRETEXAMPLE",
		}}
		_, err := p.NewCatalog(context.Background())
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), conns.Load(), "catalogs reuse one connection pool instead of leaving one open each")
}
