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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func validate(model icebergProviderModel) diag.Diagnostics {
	var diags diag.Diagnostics
	validateSigV4Config(model, &diags)

	return diags
}

func TestValidateSigV4RejectsOneOfCredentialPair(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled:     types.BoolValue(true),
		SigV4Region:      types.StringValue("us-east-1"),
		SigV4AccessKeyID: types.StringValue("AKID"),
	})

	assert.True(t, diags.HasError(), "one credential without its pair must error")
}

func TestValidateSigV4RejectsSessionTokenWithoutKeys(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled:      types.BoolValue(true),
		SigV4Region:       types.StringValue("us-east-1"),
		SigV4SessionToken: types.StringValue("STS"),
	})

	assert.True(t, diags.HasError(), "session token requires the static key pair")
}

func TestValidateSigV4AcceptsStaticCredentialsWithoutRegion(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled:         types.BoolValue(true),
		SigV4AccessKeyID:     types.StringValue("AKID"),
		SigV4SecretAccessKey: types.StringValue("SECRET"),
	})

	assert.False(t, diags.HasError(), "the region may come from the AWS environment at configure time")
}

func TestValidateSigV4AcceptsBearerTokenWithSigning(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled: types.BoolValue(true),
		SigV4Region:  types.StringValue("us-east-1"),
		Token:        types.StringValue("bearer-xyz"),
	})

	assert.False(t, diags.HasError(), "a bearer token is sent as Original-Authorization under sigv4")
}

func TestValidateSigV4WarnsWhenOptionsSetButDisabled(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Region:          types.StringValue("us-east-1"),
		SigV4AccessKeyID:     types.StringValue("AKID"),
		SigV4SecretAccessKey: types.StringValue("SECRET"),
	})

	assert.False(t, diags.HasError(), "unset sigv4_enabled is a warning, not an error")
	assert.NotEmpty(t, diags.Warnings(), "ignored sigv4 options should warn")
}

func TestValidateSigV4AcceptsCompleteStaticConfig(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled:         types.BoolValue(true),
		SigV4Region:          types.StringValue("us-east-1"),
		SigV4SigningName:     types.StringValue("glue"),
		SigV4AccessKeyID:     types.StringValue("AKID"),
		SigV4SecretAccessKey: types.StringValue("SECRET"),
	})

	assert.False(t, diags.HasError(), "a complete static config is valid")
	assert.Empty(t, diags.Warnings())
}

func TestValidateSigV4AcceptsAmbientCredentialConfig(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled: types.BoolValue(true),
		SigV4Region:  types.StringValue("us-east-1"),
	})

	assert.False(t, diags.HasError(), "enabled with a region but no static keys is valid (ambient chain)")
}

func TestValidateSigV4SkipsUnknownSecretKey(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled:         types.BoolValue(true),
		SigV4Region:          types.StringValue("us-east-1"),
		SigV4AccessKeyID:     types.StringValue("AKID"),
		SigV4SecretAccessKey: types.StringUnknown(),
	})

	assert.False(t, diags.HasError(), "an unknown secret resolved at apply must not fail plan-time validation")
}

func TestValidateSigV4SkipsUnknownRegion(t *testing.T) {
	diags := validate(icebergProviderModel{
		SigV4Enabled:         types.BoolValue(true),
		SigV4Region:          types.StringUnknown(),
		SigV4AccessKeyID:     types.StringValue("AKID"),
		SigV4SecretAccessKey: types.StringValue("SECRET"),
	})

	assert.False(t, diags.HasError(), "an unknown region resolved at apply must not fail plan-time validation")
}
