package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &branchRestrictionResource{}
var _ resource.ResourceWithImportState = &branchRestrictionResource{}

func NewBranchRestrictionResource() resource.Resource {
	return &branchRestrictionResource{}
}

type branchRestrictionResource struct {
	client *bitbucket.Client
}

type branchRestrictionResourceModel struct {
	ID              types.String `tfsdk:"id"`
	RepositorySlug  types.String `tfsdk:"repository_slug"`
	RestrictionID   types.Int64  `tfsdk:"restriction_id"`
	Kind            types.String `tfsdk:"kind"`
	BranchMatchKind types.String `tfsdk:"branch_match_kind"`
	Pattern         types.String `tfsdk:"pattern"`
	Value           types.Int64  `tfsdk:"value"`
	Users           types.List   `tfsdk:"users"`
	Groups          types.List   `tfsdk:"groups"`
}

func (r *branchRestrictionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_branch_restriction"
}

func (r *branchRestrictionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A branch restriction rule on a Bitbucket Cloud repository.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Import/state identifier, formatted as \"workspace/repository_slug/restriction_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository this restriction applies to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"restriction_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Bitbucket-assigned ID of the restriction.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"kind": schema.StringAttribute{
				Required:    true,
				Description: "Restriction type, e.g. \"push\", \"delete\", \"force\", \"restrict_merges\", \"require_approvals_to_merge\", \"require_passing_builds_to_merge\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"branch_match_kind": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "\"glob\" (use pattern) or \"branching_model\". Defaults to \"glob\".",
			},
			"pattern": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Branch name or glob pattern the restriction applies to, e.g. \"main\" or \"release/*\".",
			},
			"value": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Numeric value for restrictions that need one, e.g. required approval count.",
			},
			"users": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "UUIDs of the users this restriction applies to, e.g. who is allowed to push directly on a \"push\" kind restriction. Leave unset for restrictions that don't scope to specific users.",
			},
			"groups": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Slugs of the workspace groups this restriction applies to, e.g. which groups are allowed to push directly on a \"push\" kind restriction. Leave unset for restrictions that don't scope to specific groups.",
			},
		},
	}
}

func (r *branchRestrictionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*bitbucket.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *bitbucket.Client")
		return
	}
	r.client = client
}

func (r *branchRestrictionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan branchRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := &bitbucket.BranchRestriction{
		Kind:            plan.Kind.ValueString(),
		BranchMatchKind: firstNonEmpty(plan.BranchMatchKind.ValueString(), "glob"),
		Pattern:         plan.Pattern.ValueString(),
		Users:           accountReferencesFromList(ctx, plan.Users),
		Groups:          groupReferencesFromList(ctx, plan.Groups),
	}
	if !plan.Value.IsNull() && !plan.Value.IsUnknown() {
		v := plan.Value.ValueInt64()
		body.Value = &v
	}

	created, err := r.client.CreateBranchRestriction(ctx, plan.RepositorySlug.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket branch restriction", err.Error())
		return
	}

	model, diags := branchRestrictionModelFromAPI(ctx, r.client.Workspace(), plan.RepositorySlug.ValueString(), created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *branchRestrictionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state branchRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	br, err := r.client.GetBranchRestriction(ctx, state.RepositorySlug.ValueString(), state.RestrictionID.ValueInt64())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket branch restriction", err.Error())
		return
	}

	model, diags := branchRestrictionModelFromAPI(ctx, r.client.Workspace(), state.RepositorySlug.ValueString(), br)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *branchRestrictionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan branchRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := &bitbucket.BranchRestriction{
		Kind:            plan.Kind.ValueString(),
		BranchMatchKind: plan.BranchMatchKind.ValueString(),
		Pattern:         plan.Pattern.ValueString(),
		Users:           accountReferencesFromList(ctx, plan.Users),
		Groups:          groupReferencesFromList(ctx, plan.Groups),
	}
	if !plan.Value.IsNull() && !plan.Value.IsUnknown() {
		v := plan.Value.ValueInt64()
		body.Value = &v
	}

	updated, err := r.client.UpdateBranchRestriction(ctx, plan.RepositorySlug.ValueString(), plan.RestrictionID.ValueInt64(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket branch restriction", err.Error())
		return
	}

	model, diags := branchRestrictionModelFromAPI(ctx, r.client.Workspace(), plan.RepositorySlug.ValueString(), updated)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *branchRestrictionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state branchRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteBranchRestriction(ctx, state.RepositorySlug.ValueString(), state.RestrictionID.ValueInt64())
	if err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket branch restriction", err.Error())
	}
}

// ImportState accepts "repository_slug/restriction_id".
func (r *branchRestrictionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/restriction_id\", got: %q", req.ID),
		)
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Import Identifier", fmt.Sprintf("restriction_id %q is not a number", parts[1]))
		return
	}

	var state branchRestrictionResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.RestrictionID = types.Int64Value(id)
	// Users/Groups must be explicitly typed nulls (not the zero value of
	// types.List{}, which carries no ElementType) so the partial state set
	// here round-trips correctly before the follow-up Read fills it in.
	state.Users = types.ListNull(types.StringType)
	state.Groups = types.ListNull(types.StringType)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func branchRestrictionModelFromAPI(ctx context.Context, workspace, repoSlug string, br *bitbucket.BranchRestriction) (branchRestrictionResourceModel, diag.Diagnostics) {
	m := branchRestrictionResourceModel{
		ID:              types.StringValue(fmt.Sprintf("%s/%s/%d", workspace, repoSlug, br.ID)),
		RepositorySlug:  types.StringValue(repoSlug),
		RestrictionID:   types.Int64Value(br.ID),
		Kind:            types.StringValue(br.Kind),
		BranchMatchKind: types.StringValue(br.BranchMatchKind),
		Pattern:         types.StringValue(br.Pattern),
	}
	if br.Value != nil {
		m.Value = types.Int64Value(*br.Value)
	} else {
		m.Value = types.Int64Null()
	}

	var diags diag.Diagnostics

	userUUIDs := make([]string, 0, len(br.Users))
	for _, u := range br.Users {
		userUUIDs = append(userUUIDs, u.UUID)
	}
	usersList, d := types.ListValueFrom(ctx, types.StringType, userUUIDs)
	diags.Append(d...)
	m.Users = usersList

	groupSlugs := make([]string, 0, len(br.Groups))
	for _, g := range br.Groups {
		groupSlugs = append(groupSlugs, g.Slug)
	}
	groupsList, d := types.ListValueFrom(ctx, types.StringType, groupSlugs)
	diags.Append(d...)
	m.Groups = groupsList

	return m, diags
}

// accountReferencesFromList converts a Terraform string list of user UUIDs
// into the AccountReference slice the Bitbucket API expects.
func accountReferencesFromList(ctx context.Context, l types.List) []bitbucket.AccountReference {
	uuids := stringListToSlice(ctx, l)
	if uuids == nil {
		return nil
	}
	out := make([]bitbucket.AccountReference, 0, len(uuids))
	for _, uuid := range uuids {
		out = append(out, bitbucket.AccountReference{UUID: uuid})
	}
	return out
}

// groupReferencesFromList converts a Terraform string list of group slugs
// into the GroupReference slice the Bitbucket API expects.
func groupReferencesFromList(ctx context.Context, l types.List) []bitbucket.GroupReference {
	slugs := stringListToSlice(ctx, l)
	if slugs == nil {
		return nil
	}
	out := make([]bitbucket.GroupReference, 0, len(slugs))
	for _, slug := range slugs {
		out = append(out, bitbucket.GroupReference{Slug: slug})
	}
	return out
}
