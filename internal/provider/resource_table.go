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
	"encoding/json"
	"errors"
	"strings"

	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	"github.com/apache/iceberg-go/table"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rscschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &icebergTableResource{}

func NewTableResource() resource.Resource {
	return &icebergTableResource{}
}

type icebergTableResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Namespace        types.List   `tfsdk:"namespace"`
	Name             types.String `tfsdk:"name"`
	Schema           types.Object `tfsdk:"schema"`
	PartitionSpec    types.Object `tfsdk:"partition_spec"`
	SortOrder        types.Object `tfsdk:"sort_order"`
	UserProperties   types.Map    `tfsdk:"user_properties"`
	ServerProperties types.Map    `tfsdk:"server_properties"`
}

type icebergTableResource struct {
	catalog  catalog.Catalog
	provider *icebergProvider
}

func (r *icebergTableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table"
}

func (r *icebergTableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rscschema.Schema{
		Description: "A resource for managing Iceberg tables.",
		Attributes: map[string]rscschema.Attribute{
			"id": rscschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace": rscschema.ListAttribute{
				Description: "The namespace of the table.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"name": rscschema.StringAttribute{
				Description: "The name of the table.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": rscschema.SingleNestedAttribute{
				Description: "The schema of the table.",
				Required:    true,
				Attributes:  tableSchemaResourceAttributes(),
			},
			"partition_spec": rscschema.SingleNestedAttribute{
				Description: "The partition spec of the table.",
				Optional:    true,
				Computed:    true,
				Attributes:  tablePartitionSpecResourceAttributes(),
			},
			"sort_order": rscschema.SingleNestedAttribute{
				Description: "The sort order of the table.",
				Optional:    true,
				Computed:    true,
				Attributes:  tableSortOrderResourceAttributes(),
			},
			"user_properties": rscschema.MapAttribute{
				Description: "User-defined properties for the table.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"server_properties": rscschema.MapAttribute{
				Description: "Properties returned by the server.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *icebergTableResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*icebergProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *icebergProvider, got: %T. Please report this issue to the provider developers.",
		)

		return
	}

	r.provider = provider
}

func (r *icebergTableResource) ConfigureCatalog(ctx context.Context, diags *diag.Diagnostics) {
	if r.catalog != nil {
		return
	}

	if r.provider == nil {
		diags.AddError(
			"Provider not configured",
			"The provider hasn't been configured before this operation",
		)

		return
	}

	if r.provider.catalogURI == "" {
		// The provider might not be fully configured yet (e.g. during plan if URI is unknown)

		return
	}

	catalog, err := r.provider.NewCatalog(ctx)
	if err != nil {
		diags.AddError(
			"Failed to create catalog",
			"Failed to create catalog: "+err.Error(),
		)

		return
	}
	r.catalog = catalog
}

func (r *icebergTableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.ConfigureCatalog(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data icebergTableResourceModel

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var namespaceName []string
	diags = data.Namespace.ElementsAs(ctx, &namespaceName, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := data.Name.ValueString()
	tableIdent := append(namespaceName, tableName)

	var schema icebergTableSchema
	diags = data.Schema.As(ctx, &schema, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Assign IDs to any omitted fields; otherwise they serialize as 0 and the
	// catalog rejects the schema for duplicate field IDs.
	schema.assignFieldIDs(0)

	tblSchema, err := schema.ToIceberg()
	if err != nil {
		resp.Diagnostics.AddError("failed to convert schema", err.Error())

		return
	}

	userProps := make(map[string]string)
	if !data.UserProperties.IsNull() && !data.UserProperties.IsUnknown() {
		diags = data.UserProperties.ElementsAs(ctx, &userProps, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	createOpts := []catalog.CreateTableOpt{
		catalog.WithProperties(userProps),
	}

	if !data.PartitionSpec.IsNull() && !data.PartitionSpec.IsUnknown() {
		var spec icebergTablePartitionSpec
		diags = data.PartitionSpec.As(ctx, &spec, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		icebergSpec, err := spec.ToIceberg()
		if err != nil {
			resp.Diagnostics.AddError("failed to convert partition spec", err.Error())

			return
		}
		createOpts = append(createOpts, catalog.WithPartitionSpec(icebergSpec))
	}

	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		var order icebergTableSortOrder
		diags = data.SortOrder.As(ctx, &order, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		icebergOrder, err := order.ToIceberg()
		if err != nil {
			resp.Diagnostics.AddError("failed to convert sort order", err.Error())

			return
		}
		createOpts = append(createOpts, catalog.WithSortOrder(icebergOrder))
	}

	tbl, err := r.catalog.CreateTable(ctx, tableIdent, tblSchema, createOpts...)
	if err != nil {
		resp.Diagnostics.AddError("failed to create table", err.Error())

		return
	}

	data.ID = types.StringValue(strings.Join(tableIdent, "."))

	r.syncTableToModel(ctx, tbl, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *icebergTableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.ConfigureCatalog(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data icebergTableResourceModel

	tflog.Info(ctx, "Reading table resource")
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var namespaceName []string
	diags = data.Namespace.ElementsAs(ctx, &namespaceName, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := data.Name.ValueString()
	tableIdent := append(namespaceName, tableName)

	tbl, err := r.catalog.LoadTable(ctx, tableIdent)
	if err != nil {
		if errors.Is(err, catalog.ErrNoSuchTable) {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("failed to load table", err.Error())

		return
	}

	r.syncTableToModel(ctx, tbl, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *icebergTableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Inside table update")
	r.ConfigureCatalog(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan, state icebergTableResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var namespaceName []string
	diags = state.Namespace.ElementsAs(ctx, &namespaceName, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := state.Name.ValueString()
	tableIdent := append(namespaceName, tableName)

	tbl, err := r.catalog.LoadTable(ctx, tableIdent)
	if err != nil {
		resp.Diagnostics.AddError("failed to load table", err.Error())

		return
	}

	requirements := []table.Requirement{
		table.AssertTableUUID(tbl.Metadata().TableUUID()),
	}

	updates := make([]table.Update, 0)

	updates = append(updates, r.calculatePropertyUpdates(ctx, &plan, &state, &resp.Diagnostics)...)
	updates = append(updates, r.calculateSchemaUpdates(ctx, &plan, &state, tbl, &resp.Diagnostics)...)
	updates = append(updates, r.calculatePartitionUpdates(ctx, &plan, tbl, &resp.Diagnostics)...)
	updates = append(updates, r.calculateSortOrderUpdates(ctx, &plan, tbl, &resp.Diagnostics)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if len(updates) > 0 {
		_, _, err = r.catalog.CommitTable(ctx, tableIdent, requirements, updates)
		if err != nil {
			resp.Diagnostics.AddError("failed to commit table updates", err.Error())

			return
		}

		// Reload the table to get the latest state
		err = tbl.Refresh(ctx)
		if err != nil {
			resp.Diagnostics.AddError("failed to refresh table after commit", err.Error())

			return
		}
	}

	r.syncTableToModel(ctx, tbl, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *icebergTableResource) calculatePropertyUpdates(ctx context.Context, plan, state *icebergTableResourceModel, diags *diag.Diagnostics) []table.Update {
	updates := make([]table.Update, 0)
	userUpdates := make(iceberg.Properties)
	removals := make([]string, 0)

	stateProps := make(map[string]string)
	if !state.UserProperties.IsNull() {
		d := state.UserProperties.ElementsAs(ctx, &stateProps, false)
		diags.Append(d...)
	}

	planProps := make(map[string]string)
	if !plan.UserProperties.IsNull() {
		d := plan.UserProperties.ElementsAs(ctx, &planProps, false)
		diags.Append(d...)
	}

	if diags.HasError() {
		return nil
	}

	for k, v := range planProps {
		if oldV, ok := stateProps[k]; !ok || oldV != v {
			userUpdates[k] = v
		}
	}

	for k := range stateProps {
		if _, ok := planProps[k]; !ok {
			removals = append(removals, k)
		}
	}

	if len(userUpdates) > 0 {
		updates = append(updates, table.NewSetPropertiesUpdate(userUpdates))
	}
	if len(removals) > 0 {
		updates = append(updates, table.NewRemovePropertiesUpdate(removals))
	}

	return updates
}

func (r *icebergTableResource) calculateSchemaUpdates(ctx context.Context, plan, state *icebergTableResourceModel, tbl *table.Table, diags *diag.Diagnostics) []table.Update {
	var planSchema, stateSchema icebergTableSchema
	d := plan.Schema.As(ctx, &planSchema, basetypes.ObjectAsOptions{})
	diags.Append(d...)
	d = state.Schema.As(ctx, &stateSchema, basetypes.ObjectAsOptions{})
	diags.Append(d...)

	if diags.HasError() {
		return nil
	}

	// Assign IDs to new fields above the table's last-column-id so they never
	// collide with an existing or previously-dropped field.
	planSchema.assignFieldIDs(int64(tbl.Metadata().LastColumnID()))

	planIceberg, err := planSchema.ToIceberg()
	if err != nil {
		diags.AddError("failed to convert plan schema", err.Error())

		return nil
	}
	stateIceberg, err := stateSchema.ToIceberg()
	if err != nil {
		diags.AddError("failed to convert state schema", err.Error())

		return nil
	}

	// Normalize by comparing the JSON of the fields list only.
	// This ignores the top-level schema-id and any other schema-level metadata
	// while ensuring every field change (name, type, id, etc.) is detected.
	planFieldsJson, _ := json.Marshal(planIceberg.Fields())
	stateFieldsJson, _ := json.Marshal(stateIceberg.Fields())

	if string(planFieldsJson) == string(stateFieldsJson) {
		return nil
	}

	// Reject changes that aren't a valid Iceberg evolution before committing.
	if err := validateSchemaEvolution(tbl.Schema(), planIceberg); err != nil {
		diags.AddError("invalid schema evolution", err.Error())

		return nil
	}

	// Find the highest existing schema ID to ensure the new one is unique.
	maxSchemaID := int64(0)
	for _, s := range tbl.Metadata().Schemas() {
		if int64(s.ID) > maxSchemaID {
			maxSchemaID = int64(s.ID)
		}
	}

	newSchemaID := maxSchemaID + 1
	planSchema.ID = types.Int64Value(newSchemaID)
	planIceberg, err = planSchema.ToIceberg()
	if err != nil {
		diags.AddError("failed to convert plan schema with new ID", err.Error())

		return nil
	}

	// Return two events:
	// 1. Add the schema with new schema ID.
	// 2. Set the table schema explicitly to that new ID.
	return []table.Update{
		table.NewAddSchemaUpdate(planIceberg),
		table.NewSetCurrentSchemaUpdate(int(newSchemaID)),
	}
}

func (r *icebergTableResource) calculatePartitionUpdates(ctx context.Context, plan *icebergTableResourceModel, tbl *table.Table, diags *diag.Diagnostics) []table.Update {
	spec := tbl.Spec()
	if plan.PartitionSpec.IsUnknown() {
		if spec.NumFields() > 0 {
			// Create a new unpartitioned spec and set it as default
			unpartitionedSpec := iceberg.NewPartitionSpec()

			return []table.Update{
				table.NewAddPartitionSpecUpdate(&unpartitionedSpec, false),
				table.NewSetDefaultSpecUpdate(-1),
			}
		}

		return nil
	}

	if plan.PartitionSpec.IsNull() {
		return nil
	}

	var planSpec icebergTablePartitionSpec
	d := plan.PartitionSpec.As(ctx, &planSpec, basetypes.ObjectAsOptions{})
	diags.Append(d...)

	if diags.HasError() {
		return nil
	}

	newIcebergSpec, err := planSpec.ToIceberg()
	if err != nil {
		diags.AddError("failed to convert partition spec", err.Error())

		return nil
	}

	// Compare with current spec
	if !spec.CompatibleWith(newIcebergSpec) {
		return []table.Update{
			table.NewAddPartitionSpecUpdate(newIcebergSpec, false),
			table.NewSetDefaultSpecUpdate(-1),
		}
	}

	return nil
}

func (r *icebergTableResource) calculateSortOrderUpdates(ctx context.Context, plan *icebergTableResourceModel, tbl *table.Table, diags *diag.Diagnostics) []table.Update {
	if plan.SortOrder.IsUnknown() {
		if tbl.SortOrder().OrderID() != 0 {
			// Create a new unsorted order and set it as default
			unsortedOrder := table.UnsortedSortOrder

			return []table.Update{
				table.NewAddSortOrderUpdate(&unsortedOrder),
				table.NewSetDefaultSortOrderUpdate(0),
			}
		}

		return nil
	}

	if plan.SortOrder.IsNull() {
		return nil
	}

	var planOrder icebergTableSortOrder
	d := plan.SortOrder.As(ctx, &planOrder, basetypes.ObjectAsOptions{})
	diags.Append(d...)

	if diags.HasError() {
		return nil
	}

	newIcebergOrder, err := planOrder.ToIceberg()
	if err != nil {
		diags.AddError("failed to convert sort order", err.Error())

		return nil
	}

	// Compare with current sort order
	if !tbl.SortOrder().Equals(newIcebergOrder) {
		return []table.Update{
			table.NewAddSortOrderUpdate(&newIcebergOrder),
			table.NewSetDefaultSortOrderUpdate(-1),
		}
	}

	return nil
}

func (r *icebergTableResource) syncTableToModel(ctx context.Context, tbl *table.Table, model *icebergTableResourceModel, diags *diag.Diagnostics) {
	meta := icebergTableMetadataFields{}
	convertIcebergTableMetadataToModel(ctx, tbl, &meta, diags)
	if diags.HasError() {
		return
	}
	model.Schema = meta.Schema
	model.PartitionSpec = meta.PartitionSpec
	model.SortOrder = meta.SortOrder
	model.ServerProperties = meta.ServerProperties

	// Update UserProperties to match reality for tracked keys
	if !model.UserProperties.IsNull() {
		planProps := make(map[string]string)
		d := model.UserProperties.ElementsAs(ctx, &planProps, false)
		diags.Append(d...)
		if diags.HasError() {
			return
		}

		managedProps := make(map[string]string)
		for k := range planProps {
			if v, ok := tbl.Properties()[k]; ok {
				managedProps[k] = v
			}
		}
		model.UserProperties, d = types.MapValueFrom(ctx, types.StringType, managedProps)
		diags.Append(d...)
	}
}

func (r *icebergTableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.ConfigureCatalog(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var data icebergTableResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var namespaceName []string
	diags = data.Namespace.ElementsAs(ctx, &namespaceName, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := data.Name.ValueString()
	tableIdent := append(namespaceName, tableName)

	err := r.catalog.DropTable(ctx, tableIdent)
	if err != nil {
		if errors.Is(err, catalog.ErrNoSuchTable) {
			// If the table is already gone, we don't need to do anything.
			return
		}
		resp.Diagnostics.AddError("failed to drop table", err.Error())

		return
	}
}

func (r *icebergTableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	// FieldsFunc is used to split by dot and filter out empty segments (e.g. "a..b" -> ["a", "b"])
	parts := strings.FieldsFunc(req.ID, func(r rune) bool {
		return r == '.'
	})

	if len(parts) < 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"The import ID should be a dot-separated full identifier (namespace + name).",
		)

		return
	}

	tableName := parts[len(parts)-1]
	namespaceParts := parts[:len(parts)-1]

	namespaceList, diags := types.ListValueFrom(ctx, types.StringType, namespaceParts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(tableName))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), namespaceList)...)
}
