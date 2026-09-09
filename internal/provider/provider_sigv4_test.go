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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureConfigServer stubs a REST catalog and records the /v1/config request headers.
func captureConfigServer(t *testing.T, auth, contentSHA, securityToken *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/config" {
			*auth = r.Header.Get("Authorization")
			*contentSHA = r.Header.Get("x-amz-content-sha256")
			*securityToken = r.Header.Get("X-Amz-Security-Token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"defaults":{},"overrides":{}}`))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestNewCatalogSignsRequestsWithSigV4(t *testing.T) {
	var auth, contentSHA, securityToken string
	srv := captureConfigServer(t, &auth, &contentSHA, &securityToken)

	p := &icebergProvider{
		catalogURI:           srv.URL,
		catalogType:          "rest",
		sigv4Enabled:         true,
		sigv4Region:          "us-east-1",
		sigv4SigningName:     "glue",
		sigv4AccessKeyID:     "AKIDEXAMPLE",
		sigv4SecretAccessKey: "SECRETEXAMPLE",
	}

	_, err := p.NewCatalog(context.Background())
	require.NoError(t, err)

	assert.Contains(t, auth, "AWS4-HMAC-SHA256",
		"config request must be signed with AWS SigV4")
	assert.Contains(t, auth, "/us-east-1/glue/aws4_request",
		"credential scope must carry the configured region and signing name")
	assert.NotEmpty(t, contentSHA,
		"SigV4 requires the x-amz-content-sha256 header")
}

func TestNewCatalogDefaultsSigningNameToExecuteAPI(t *testing.T) {
	var auth, contentSHA, securityToken string
	srv := captureConfigServer(t, &auth, &contentSHA, &securityToken)

	p := &icebergProvider{
		catalogURI:           srv.URL,
		catalogType:          "rest",
		sigv4Enabled:         true,
		sigv4Region:          "us-east-1",
		sigv4AccessKeyID:     "AKIDEXAMPLE",
		sigv4SecretAccessKey: "SECRETEXAMPLE",
	}

	_, err := p.NewCatalog(context.Background())
	require.NoError(t, err)

	assert.Contains(t, auth, "/us-east-1/execute-api/aws4_request",
		"an omitted signing name must default to execute-api")
}

func TestNewCatalogSignsWithSessionToken(t *testing.T) {
	var auth, contentSHA, securityToken string
	srv := captureConfigServer(t, &auth, &contentSHA, &securityToken)

	p := &icebergProvider{
		catalogURI:           srv.URL,
		catalogType:          "rest",
		sigv4Enabled:         true,
		sigv4Region:          "us-east-1",
		sigv4SigningName:     "glue",
		sigv4AccessKeyID:     "AKIDEXAMPLE",
		sigv4SecretAccessKey: "SECRETEXAMPLE",
		sigv4SessionToken:    "SESSIONTOKEN",
	}

	_, err := p.NewCatalog(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "SESSIONTOKEN", securityToken,
		"temporary credentials must send X-Amz-Security-Token")
}

func TestNewCatalogWithoutSigV4SendsNoSignature(t *testing.T) {
	var auth, contentSHA, securityToken string
	srv := captureConfigServer(t, &auth, &contentSHA, &securityToken)

	p := &icebergProvider{catalogURI: srv.URL, catalogType: "rest"}

	_, err := p.NewCatalog(context.Background())
	require.NoError(t, err)

	assert.Empty(t, auth, "plain config must not send an Authorization header")
	assert.Empty(t, contentSHA, "plain config must not send x-amz-content-sha256")
}

// captureConfigHeaders stubs a REST catalog and records the /v1/config request headers.
func captureConfigHeaders(t *testing.T, into *http.Header) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/config" {
			*into = r.Header.Clone()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"defaults":{},"overrides":{}}`))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestNewCatalogRelocatesBearerTokenUnderSigV4(t *testing.T) {
	var headers http.Header
	srv := captureConfigHeaders(t, &headers)

	p := &icebergProvider{
		catalogURI:           srv.URL,
		catalogType:          "rest",
		token:                "bearer-example",
		sigv4Enabled:         true,
		sigv4Region:          "us-east-1",
		sigv4AccessKeyID:     "AKIDEXAMPLE",
		sigv4SecretAccessKey: "SECRETEXAMPLE",
	}

	_, err := p.NewCatalog(context.Background())
	require.NoError(t, err)

	assert.Contains(t, headers.Get("Authorization"), "AWS4-HMAC-SHA256",
		"SigV4 owns the Authorization header")
	assert.Equal(t, "Bearer bearer-example", headers.Get("Original-Authorization"),
		"the bearer token moves to Original-Authorization, as the Java client does")
	assert.Contains(t, headers.Get("Authorization"), "original-authorization",
		"the relocated header is part of the signed headers")
}

func TestNewCatalogResolvesRegionFromEnvironmentForStaticCredentials(t *testing.T) {
	var headers http.Header
	srv := captureConfigHeaders(t, &headers)
	t.Setenv("AWS_REGION", "eu-west-1")

	p := &icebergProvider{
		catalogURI:           srv.URL,
		catalogType:          "rest",
		sigv4Enabled:         true,
		sigv4AccessKeyID:     "AKIDEXAMPLE",
		sigv4SecretAccessKey: "SECRETEXAMPLE",
	}

	_, err := p.NewCatalog(context.Background())
	require.NoError(t, err)

	assert.Contains(t, headers.Get("Authorization"), "/eu-west-1/execute-api/aws4_request",
		"an omitted sigv4_region falls back to the AWS environment")
}

func TestNewCatalogRejectsStaticCredentialsWithoutAnyRegion(t *testing.T) {
	var headers http.Header
	srv := captureConfigHeaders(t, &headers)
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/credentials")

	p := &icebergProvider{
		catalogURI:           srv.URL,
		catalogType:          "rest",
		sigv4Enabled:         true,
		sigv4AccessKeyID:     "AKIDEXAMPLE",
		sigv4SecretAccessKey: "SECRETEXAMPLE",
	}

	_, err := p.NewCatalog(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sigv4_region")
}
