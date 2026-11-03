package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &defaultReviewerResource{}
var _ resource.ResourceWithImportState = &defaultReviewerResource{}

func NewDefaultReviewerResource() resource.Resource {
	return &defaultReviewerResource{}
}

type defaultReviewerResource struct {
	client *bitbucket.Client
}

type defaultReviewerResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepositorySlug types.String `tfsdk:"repository_slug"`
	User           types.String `tfsdk:"user"`
}

func (r *defaultReviewerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_default_reviewer"
}

func (r *defaultReviewerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adds a user as a default reviewer on a Bitbucket Cloud repository: they're automatically added as a reviewer on every new pull request. One resource per user; add one of these per default reviewer you want.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug/user\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository to add the default reviewer to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Required:    true,
				Description: "UUID or Atlassian account ID of the user to add as a default reviewer.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *defaultReviewerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *defaultReviewerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan defaultReviewerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.AddDefaultReviewer(ctx, plan.RepositorySlug.ValueString(), plan.User.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error adding Bitbucket default reviewer", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, defaultReviewerModel(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.User.ValueString()))...)
}

func (r *defaultReviewerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state defaultReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.GetDefaultReviewer(ctx, state.RepositorySlug.ValueString(), state.User.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket default reviewer", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, defaultReviewerModel(r.client.Workspace(), state.RepositorySlug.ValueString(), state.User.ValueString()))...)
}

func (r *defaultReviewerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan defaultReviewerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *defaultReviewerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state defaultReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveDefaultReviewer(ctx, state.RepositorySlug.ValueString(), state.User.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing Bitbucket default reviewer", err.Error())
	}
}

// ImportState accepts "repository_slug/user".
func (r *defaultReviewerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/user\", got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, defaultReviewerResourceModel{
		RepositorySlug: types.StringValue(parts[0]),
		User:           types.StringValue(parts[1]),
	})...)
}

func defaultReviewerModel(workspace, repoSlug, user string) defaultReviewerResourceModel {
	return defaultReviewerResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, user)),
		RepositorySlug: types.StringValue(repoSlug),
		User:           types.StringValue(user),
	}
}
