package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &repositoryUserPermissionResource{}
var _ resource.ResourceWithImportState = &repositoryUserPermissionResource{}

func NewRepositoryUserPermissionResource() resource.Resource {
	return &repositoryUserPermissionResource{}
}

type repositoryUserPermissionResource struct {
	client *bitbucket.Client
}

type repositoryUserPermissionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepositorySlug types.String `tfsdk:"repository_slug"`
	User           types.String `tfsdk:"user"`
	Permission     types.String `tfsdk:"permission"`
}

func (r *repositoryUserPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_user_permission"
}

func (r *repositoryUserPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a user a permission level (read/write/admin) on a Bitbucket Cloud repository.",
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
				Description: "Slug of the repository to grant access to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Required:    true,
				Description: "UUID (e.g. \"{11111111-2222-3333-4444-555555555555}\") or Atlassian account ID of the user to grant access to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permission": schema.StringAttribute{
				Required:    true,
				Description: "Permission level: \"read\", \"write\" or \"admin\".",
				Validators: []validator.String{
					stringvalidator.OneOf("read", "write", "admin"),
				},
			},
		},
	}
}

func (r *repositoryUserPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *repositoryUserPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan repositoryUserPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetRepositoryUserPermission(ctx, plan.RepositorySlug.ValueString(), plan.User.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error granting Bitbucket repository user permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryUserPermissionModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.User.ValueString(), granted))...)
}

func (r *repositoryUserPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state repositoryUserPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.GetRepositoryUserPermission(ctx, state.RepositorySlug.ValueString(), state.User.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket repository user permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryUserPermissionModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), state.User.ValueString(), granted))...)
}

func (r *repositoryUserPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan repositoryUserPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetRepositoryUserPermission(ctx, plan.RepositorySlug.ValueString(), plan.User.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket repository user permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryUserPermissionModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.User.ValueString(), granted))...)
}

func (r *repositoryUserPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state repositoryUserPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRepositoryUserPermission(ctx, state.RepositorySlug.ValueString(), state.User.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking Bitbucket repository user permission", err.Error())
	}
}

// ImportState accepts "repository_slug/user".
func (r *repositoryUserPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/user\", got: %q", req.ID),
		)
		return
	}

	var state repositoryUserPermissionResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.User = types.StringValue(parts[1])
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func repositoryUserPermissionModelFromAPI(workspace, repoSlug, user string, p *bitbucket.RepositoryUserPermission) repositoryUserPermissionResourceModel {
	return repositoryUserPermissionResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, user)),
		RepositorySlug: types.StringValue(repoSlug),
		User:           types.StringValue(user),
		Permission:     types.StringValue(p.Permission),
	}
}
