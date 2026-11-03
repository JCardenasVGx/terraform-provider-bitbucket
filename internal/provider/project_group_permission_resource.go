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

var _ resource.Resource = &projectGroupPermissionResource{}
var _ resource.ResourceWithImportState = &projectGroupPermissionResource{}

func NewProjectGroupPermissionResource() resource.Resource {
	return &projectGroupPermissionResource{}
}

type projectGroupPermissionResource struct {
	client *bitbucket.Client
}

type projectGroupPermissionResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ProjectKey types.String `tfsdk:"project_key"`
	GroupSlug  types.String `tfsdk:"group_slug"`
	Permission types.String `tfsdk:"permission"`
}

func (r *projectGroupPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_group_permission"
}

func (r *projectGroupPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a workspace group a permission level (read/write/admin) on a Bitbucket Cloud project, applying to every repository inside it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/project_key/group_slug\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_key": schema.StringAttribute{
				Required:    true,
				Description: "Key of the project to grant access to.",
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

func (r *projectGroupPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectGroupPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectGroupPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetProjectGroupPermission(ctx, plan.ProjectKey.ValueString(), plan.GroupSlug.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error granting Bitbucket project group permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectGroupPermissionModelFromAPI(r.client.Workspace(), plan.ProjectKey.ValueString(), plan.GroupSlug.ValueString(), granted))...)
}

func (r *projectGroupPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectGroupPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.GetProjectGroupPermission(ctx, state.ProjectKey.ValueString(), state.GroupSlug.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket project group permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectGroupPermissionModelFromAPI(r.client.Workspace(), state.ProjectKey.ValueString(), state.GroupSlug.ValueString(), granted))...)
}

func (r *projectGroupPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectGroupPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetProjectGroupPermission(ctx, plan.ProjectKey.ValueString(), plan.GroupSlug.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket project group permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectGroupPermissionModelFromAPI(r.client.Workspace(), plan.ProjectKey.ValueString(), plan.GroupSlug.ValueString(), granted))...)
}

func (r *projectGroupPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectGroupPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteProjectGroupPermission(ctx, state.ProjectKey.ValueString(), state.GroupSlug.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking Bitbucket project group permission", err.Error())
	}
}

// ImportState accepts "project_key/group_slug".
func (r *projectGroupPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"project_key/group_slug\", got: %q", req.ID),
		)
		return
	}

	var state projectGroupPermissionResourceModel
	state.ProjectKey = types.StringValue(parts[0])
	state.GroupSlug = types.StringValue(parts[1])
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func projectGroupPermissionModelFromAPI(workspace, projectKey, groupSlug string, p *bitbucket.ProjectGroupPermission) projectGroupPermissionResourceModel {
	return projectGroupPermissionResourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, projectKey, groupSlug)),
		ProjectKey: types.StringValue(projectKey),
		GroupSlug:  types.StringValue(groupSlug),
		Permission: types.StringValue(p.Permission),
	}
}
