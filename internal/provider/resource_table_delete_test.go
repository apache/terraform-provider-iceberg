// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deleteTable runs the table resource's Delete against a stub REST catalog and
// returns the purgeRequested value of the drop request.
func deleteTable(t *testing.T, purgeOnDestroy types.Bool) string {
	t.Helper()
	ctx := context.Background()

	purgeRequested := "<no drop request>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/config":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"defaults":{},"overrides":{}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/namespaces/db/tables/events":
			purgeRequested = r.URL.Query().Get("purgeRequested")
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := &icebergTableResource{provider: &icebergProvider{catalogURI: srv.URL, catalogType: "rest"}}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	require.False(t, state.SetAttribute(ctx, path.Root("namespace"), []string{"db"}).HasError())
	require.False(t, state.SetAttribute(ctx, path.Root("name"), "events").HasError())
	require.False(t, state.SetAttribute(ctx, path.Root("purge_on_destroy"), purgeOnDestroy).HasError())

	resp := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	return purgeRequested
}

func TestTableDeletePurgesWhenPurgeOnDestroy(t *testing.T) {
	assert.Equal(t, "true", deleteTable(t, types.BoolValue(true)))
}

func TestTableDeleteDropsWithoutPurgeByDefault(t *testing.T) {
	assert.Equal(t, "false", deleteTable(t, types.BoolValue(false)))
	assert.Equal(t, "false", deleteTable(t, types.BoolNull()), "state written before the attribute existed")
}
