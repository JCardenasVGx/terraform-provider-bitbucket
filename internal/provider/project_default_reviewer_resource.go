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

var _ resource.Resource = &projectDefaultReviewerResource{}
var _ resource.ResourceWithImportState = &projectDefaultReviewerResource{}

func NewProjectDefaultReviewerResource() resource.Resource {
	return &projectDefaultReviewerResource{}
}

type projectDefaultReviewerResource struct {
	client *bitbucket.Client
}

type projectDefaultReviewerResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ProjectKey types.String `tfsdk:"project_key"`
	User       types.String `tfsdk:"user"`
}

func (r *projectDefaultReviewerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_default_reviewer"
}

func (r *projectDefaultReviewerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adds a user as a default reviewer on every repository in a Bitbucket Cloud project. One resource per user; add one of these per default reviewer you want.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/project_key/user\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_key": schema.StringAttribute{
				Required:    true,
				Description: "Key of the project to add the default reviewer to.",
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

func (r *projectDefaultReviewerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectDefaultReviewerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectDefaultReviewerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.AddProjectDefaultReviewer(ctx, plan.ProjectKey.ValueString(), plan.User.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error adding Bitbucket project default reviewer", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectDefaultReviewerModel(r.client.Workspace(), plan.ProjectKey.ValueString(), plan.User.ValueString()))...)
}

func (r *projectDefaultReviewerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectDefaultReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.GetProjectDefaultReviewer(ctx, state.ProjectKey.ValueString(), state.User.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket project default reviewer", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectDefaultReviewerModel(r.client.Workspace(), state.ProjectKey.ValueString(), state.User.ValueString()))...)
}

func (r *projectDefaultReviewerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectDefaultReviewerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectDefaultReviewerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectDefaultReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveProjectDefaultReviewer(ctx, state.ProjectKey.ValueString(), state.User.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing Bitbucket project default reviewer", err.Error())
	}
}

// ImportState accepts "project_key/user".
func (r *projectDefaultReviewerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"project_key/user\", got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, projectDefaultReviewerResourceModel{
		ProjectKey: types.StringValue(parts[0]),
		User:       types.StringValue(parts[1]),
	})...)
}

func projectDefaultReviewerModel(workspace, projectKey, user string) projectDefaultReviewerResourceModel {
	return projectDefaultReviewerResourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, projectKey, user)),
		ProjectKey: types.StringValue(projectKey),
		User:       types.StringValue(user),
	}
}
