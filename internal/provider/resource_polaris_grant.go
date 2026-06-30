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
	"strings"

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
	_ resource.Resource                = &polarisGrantResource{}
	_ resource.ResourceWithImportState = &polarisGrantResource{}
)

func NewPolarisGrantResource() resource.Resource {
	return &polarisGrantResource{}
}

type polarisGrantResource struct {
	provider         *icebergProvider
	managementClient *polarisManagementClient
}

type grantSecurableKind string

const (
	grantKindCatalog   grantSecurableKind = "CATALOG"
	grantKindNamespace grantSecurableKind = "NAMESPACE"
	grantKindTable     grantSecurableKind = "TABLE"
	grantKindView      grantSecurableKind = "VIEW"
)

type polarisGrantResourceModel struct {
	ID              types.String `tfsdk:"id"`
	CatalogName     types.String `tfsdk:"catalog_name"`
	CatalogRoleName types.String `tfsdk:"catalog_role_name"`
	Privilege       types.String `tfsdk:"privilege"`
	Catalog         types.String `tfsdk:"catalog"`
	Namespace       types.List   `tfsdk:"namespace"`
	Table           types.String `tfsdk:"table"`
	View            types.String `tfsdk:"view"`
	Cascade         types.Bool   `tfsdk:"cascade"`
}

// resolveSecurable returns the kind of securable resource this grant targets,
// along with the parsed namespace parts and object name (for table/view).
func (m *polarisGrantResourceModel) resolveSecurable(ctx context.Context) (grantSecurableKind, []string, string, error) {
	ns := listToStringSlice(ctx, m.Namespace)

	hasCatalog := !m.Catalog.IsNull() && !m.Catalog.IsUnknown() && m.Catalog.ValueString() != ""
	hasNs := len(ns) > 0
	hasTable := !m.Table.IsNull() && !m.Table.IsUnknown() && m.Table.ValueString() != ""
	hasView := !m.View.IsNull() && !m.View.IsUnknown() && m.View.ValueString() != ""

	set := 0
	if hasCatalog {
		set++
	}
	if hasNs && !hasTable && !hasView {
		set++
	}
	if hasTable {
		set++
	}
	if hasView {
		set++
	}

	if set == 0 {
		return "", nil, "", errors.New("one of catalog, namespace, table, or view must be set")
	}
	if set > 1 {
		return "", nil, "", errors.New("catalog, namespace, table, and view are mutually exclusive")
	}
	if hasTable && !hasNs {
		return "", nil, "", errors.New("table requires namespace to be set")
	}
	if hasView && !hasNs {
		return "", nil, "", errors.New("view requires namespace to be set")
	}

	switch {
	case hasCatalog:
		return grantKindCatalog, nil, "", nil
	case hasTable:
		return grantKindTable, ns, m.Table.ValueString(), nil
	case hasView:
		return grantKindView, ns, m.View.ValueString(), nil
	default:
		return grantKindNamespace, ns, "", nil
	}
}

func (r *polarisGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_polaris_grant"
}

func (r *polarisGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a privilege on a securable object to a catalog role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"catalog_name": schema.StringAttribute{
				Description: "The catalog containing the role.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog_role_name": schema.StringAttribute{
				Description: "The catalog role receiving the grant.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"privilege": schema.StringAttribute{
				Description: "The privilege to grant.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog": schema.StringAttribute{
				Description: "Grant on the whole catalog. Mutually exclusive with namespace/table/view.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"namespace": schema.ListAttribute{
				Description: "Namespace to grant on (e.g. [\"ns1\", \"subns\"]). Required if table or view is set.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					requiresReplaceIfListChanged(),
				},
			},
			"table": schema.StringAttribute{
				Description: "Table name to grant on. Requires namespace.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"view": schema.StringAttribute{
				Description: "View name to grant on. Requires namespace.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cascade": schema.BoolAttribute{
				Description: "Cascade revocation to subresources (only used on destroy).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *polarisGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	provider, ok := req.ProviderData.(*icebergProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *icebergProvider, got a different type.",
		)

		return
	}
	r.provider = provider
}

func (r *polarisGrantResource) ensureManagementClient(ctx context.Context, diags *diag.Diagnostics) {
	if r.managementClient != nil {
		return
	}
	if r.provider == nil {
		diags.AddError("Provider not configured", "The provider hasn't been configured before this operation")

		return
	}
	client, err := r.provider.newPolarisManagementClient()
	if err != nil {
		diags.AddError("Failed to create Polaris management API client", err.Error())

		return
	}
	r.managementClient = client
}

func (r *polarisGrantResource) validateSecurableType(ctx context.Context, data *polarisGrantResourceModel) error {
	_, _, _, err := data.resolveSecurable(ctx)

	return err
}

func (r *polarisGrantResource) buildGrantResource(ctx context.Context, data *polarisGrantResourceModel) polarisGrantResourceBody {
	var grant polarisGrantResourceBody

	kind, ns, name, _ := data.resolveSecurable(ctx)
	grant.Privilege = data.Privilege.ValueString()

	switch kind {
	case grantKindCatalog:
		grant.Type = "catalog"
	case grantKindTable:
		grant.Type = "table"
		grant.Namespace = ns
		grant.TableName = name
	case grantKindView:
		grant.Type = "view"
		grant.Namespace = ns
		grant.ViewName = name
	case grantKindNamespace:
		grant.Type = "namespace"
		grant.Namespace = ns
	}

	return grant
}

func (r *polarisGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.validateSecurableType(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Invalid grant configuration", err.Error())

		return
	}

	grant := r.buildGrantResource(ctx, &data)

	tflog.Info(ctx, "Creating grant", map[string]any{
		"catalog":   data.CatalogName.ValueString(),
		"role":      data.CatalogRoleName.ValueString(),
		"type":      grant.Type,
		"privilege": grant.Privilege,
	})

	if err := r.managementClient.AddGrant(ctx, data.CatalogName.ValueString(), data.CatalogRoleName.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to add grant", err.Error())

		return
	}

	data.ID = types.StringValue(buildGrantID(ctx, &data))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grants, err := r.managementClient.ListGrants(ctx, data.CatalogName.ValueString(), data.CatalogRoleName.ValueString())
	if err != nil {
		if isPolarisNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to list grants", err.Error())

		return
	}

	if !findMatchingGrant(ctx, grants, &data) {
		resp.State.RemoveResource(ctx)

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No-op update: cascade is the only mutable attribute, and it's only
	// consumed on Delete. Just return the plan state as-is.
	var data polarisGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *polarisGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ensureManagementClient(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data polarisGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := r.buildGrantResource(ctx, &data)
	cascade := data.Cascade.ValueBool()

	tflog.Info(ctx, "Revoking grant", map[string]any{
		"catalog":   data.CatalogName.ValueString(),
		"role":      data.CatalogRoleName.ValueString(),
		"type":      grant.Type,
		"privilege": grant.Privilege,
		"cascade":   cascade,
	})

	err := r.managementClient.RevokeGrant(ctx, data.CatalogName.ValueString(), data.CatalogRoleName.ValueString(), grant, cascade)
	if err != nil && !isPolarisNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to revoke grant", err.Error())
	}
}

func (r *polarisGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Format: {catalog_name}/{catalog_role_name}/{securable_type}/{resource_path}/{privilege}
	parts := strings.SplitN(req.ID, "/", 5)
	if len(parts) != 5 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: {catalog}/{role}/{type}/{resource_path}/{privilege}")

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role_name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[4])...)

	securableType := parts[2]
	resourcePath := parts[3]

	switch securableType {
	case "CATALOG":
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	case "NAMESPACE":
		nsParts := strings.Split(resourcePath, ".")
		nsList, diags := types.ListValueFrom(ctx, types.StringType, nsParts)
		resp.Diagnostics.Append(diags...)
		if !diags.HasError() {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
		}
	case "TABLE":
		parts := strings.SplitN(resourcePath, ".", 2)
		if len(parts) == 2 {
			nsParts := strings.Split(parts[0], ".")
			nsList, diags := types.ListValueFrom(ctx, types.StringType, nsParts)
			resp.Diagnostics.Append(diags...)
			if !diags.HasError() {
				resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table"), parts[1])...)
		}
	case "VIEW":
		vParts := strings.SplitN(resourcePath, ".", 2)
		if len(vParts) == 2 {
			nsParts := strings.Split(vParts[0], ".")
			nsList, diags := types.ListValueFrom(ctx, types.StringType, nsParts)
			resp.Diagnostics.Append(diags...)
			if !diags.HasError() {
				resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("view"), vParts[1])...)
		}
	default:
		resp.Diagnostics.AddError("Invalid securable type in import ID", "Expected CATALOG, NAMESPACE, TABLE, or VIEW")
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func buildGrantID(ctx context.Context, data *polarisGrantResourceModel) string {
	catalogName := data.CatalogName.ValueString()
	roleName := data.CatalogRoleName.ValueString()
	privilege := data.Privilege.ValueString()

	kind, ns, name, _ := data.resolveSecurable(ctx)

	securableType := string(kind)
	resourcePath := "null"

	switch kind {
	case grantKindTable:
		resourcePath = strings.Join(ns, ".") + "." + name
	case grantKindView:
		resourcePath = strings.Join(ns, ".") + "." + name
	case grantKindNamespace:
		resourcePath = strings.Join(ns, ".")
	}

	return fmt.Sprintf("%s/%s/%s/%s/%s", catalogName, roleName, securableType, resourcePath, privilege)
}

func findMatchingGrant(ctx context.Context, grants *polarisGrantResources, data *polarisGrantResourceModel) bool {
	privilege := data.Privilege.ValueString()
	kind, ns, name, _ := data.resolveSecurable(ctx)

	for _, g := range grants.Grants {
		if g.Privilege != privilege {
			continue
		}

		switch kind {
		case grantKindCatalog:
			if g.Type == "catalog" {
				return true
			}
		case grantKindTable:
			if g.Type == "table" && g.TableName == name {
				return true
			}
		case grantKindView:
			if g.Type == "view" && g.ViewName == name {
				return true
			}
		case grantKindNamespace:
			if g.Type == "namespace" && stringSlicesEqual(g.Namespace, ns) {
				return true
			}
		}
	}

	return false
}

func listToStringSlice(ctx context.Context, l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	_ = l.ElementsAs(ctx, &out, false)

	return out
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
