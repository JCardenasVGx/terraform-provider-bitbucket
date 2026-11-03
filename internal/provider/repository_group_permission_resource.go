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

var _ resource.Resource = &repositoryGroupPermissionResource{}
var _ resource.ResourceWithImportState = &repositoryGroupPermissionResource{}

func NewRepositoryGroupPermissionResource() resource.Resource {
	return &repositoryGroupPermissionResource{}
}

type repositoryGroupPermissionResource struct {
	client *bitbucket.Client
}

type repositoryGroupPermissionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepositorySlug types.String `tfsdk:"repository_slug"`
	GroupSlug      types.String `tfsdk:"group_slug"`
	Permission     types.String `tfsdk:"permission"`
}

func (r *repositoryGroupPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_group_permission"
}

func (r *repositoryGroupPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a workspace group a permission level (read/write/admin) on a Bitbucket Cloud repository.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug/group_slug\".",
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
			"group_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the workspace group to grant access to.",
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

func (r *repositoryGroupPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *repositoryGroupPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan repositoryGroupPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetRepositoryGroupPermission(ctx, plan.RepositorySlug.ValueString(), plan.GroupSlug.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error granting Bitbucket repository group permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryGroupPermissionModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.GroupSlug.ValueString(), granted))...)
}

func (r *repositoryGroupPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state repositoryGroupPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.GetRepositoryGroupPermission(ctx, state.RepositorySlug.ValueString(), state.GroupSlug.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket repository group permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryGroupPermissionModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), state.GroupSlug.ValueString(), granted))...)
}

func (r *repositoryGroupPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan repositoryGroupPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetRepositoryGroupPermission(ctx, plan.RepositorySlug.ValueString(), plan.GroupSlug.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket repository group permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryGroupPermissionModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.GroupSlug.ValueString(), granted))...)
}

func (r *repositoryGroupPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state repositoryGroupPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRepositoryGroupPermission(ctx, state.RepositorySlug.ValueString(), state.GroupSlug.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking Bitbucket repository group permission", err.Error())
	}
}

// ImportState accepts "repository_slug/group_slug".
func (r *repositoryGroupPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/group_slug\", got: %q", req.ID),
		)
		return
	}

	var state repositoryGroupPermissionResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.GroupSlug = types.StringValue(parts[1])
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func repositoryGroupPermissionModelFromAPI(workspace, repoSlug, groupSlug string, p *bitbucket.RepositoryGroupPermission) repositoryGroupPermissionResourceModel {
	return repositoryGroupPermissionResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, groupSlug)),
		RepositorySlug: types.StringValue(repoSlug),
		GroupSlug:      types.StringValue(groupSlug),
		Permission:     types.StringValue(p.Permission),
	}
}
