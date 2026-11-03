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

var _ resource.Resource = &projectUserPermissionResource{}
var _ resource.ResourceWithImportState = &projectUserPermissionResource{}

func NewProjectUserPermissionResource() resource.Resource {
	return &projectUserPermissionResource{}
}

type projectUserPermissionResource struct {
	client *bitbucket.Client
}

type projectUserPermissionResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ProjectKey types.String `tfsdk:"project_key"`
	User       types.String `tfsdk:"user"`
	Permission types.String `tfsdk:"permission"`
}

func (r *projectUserPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_user_permission"
}

func (r *projectUserPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a user a permission level (read/write/admin) on a Bitbucket Cloud project, applying to every repository inside it.",
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
				Description: "Key of the project to grant access to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Required:    true,
				Description: "UUID or Atlassian account ID of the user to grant access to.",
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

func (r *projectUserPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectUserPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectUserPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetProjectUserPermission(ctx, plan.ProjectKey.ValueString(), plan.User.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error granting Bitbucket project user permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectUserPermissionModelFromAPI(r.client.Workspace(), plan.ProjectKey.ValueString(), plan.User.ValueString(), granted))...)
}

func (r *projectUserPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectUserPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.GetProjectUserPermission(ctx, state.ProjectKey.ValueString(), state.User.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket project user permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectUserPermissionModelFromAPI(r.client.Workspace(), state.ProjectKey.ValueString(), state.User.ValueString(), granted))...)
}

func (r *projectUserPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectUserPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	granted, err := r.client.SetProjectUserPermission(ctx, plan.ProjectKey.ValueString(), plan.User.ValueString(), plan.Permission.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket project user permission", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectUserPermissionModelFromAPI(r.client.Workspace(), plan.ProjectKey.ValueString(), plan.User.ValueString(), granted))...)
}

func (r *projectUserPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectUserPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteProjectUserPermission(ctx, state.ProjectKey.ValueString(), state.User.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking Bitbucket project user permission", err.Error())
	}
}

// ImportState accepts "project_key/user".
func (r *projectUserPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"project_key/user\", got: %q", req.ID),
		)
		return
	}

	var state projectUserPermissionResourceModel
	state.ProjectKey = types.StringValue(parts[0])
	state.User = types.StringValue(parts[1])
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func projectUserPermissionModelFromAPI(workspace, projectKey, user string, p *bitbucket.ProjectUserPermission) projectUserPermissionResourceModel {
	return projectUserPermissionResourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, projectKey, user)),
		ProjectKey: types.StringValue(projectKey),
		User:       types.StringValue(user),
		Permission: types.StringValue(p.Permission),
	}
}
