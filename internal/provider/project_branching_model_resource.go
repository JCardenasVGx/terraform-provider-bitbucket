package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &projectBranchingModelResource{}
var _ resource.ResourceWithImportState = &projectBranchingModelResource{}

func NewProjectBranchingModelResource() resource.Resource {
	return &projectBranchingModelResource{}
}

type projectBranchingModelResource struct {
	client *bitbucket.Client
}

type projectBranchingModelResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	ProjectKey               types.String `tfsdk:"project_key"`
	DevelopmentBranch        types.String `tfsdk:"development_branch"`
	DevelopmentUseMainbranch types.Bool   `tfsdk:"development_use_mainbranch"`
	ProductionEnabled        types.Bool   `tfsdk:"production_enabled"`
	ProductionBranch         types.String `tfsdk:"production_branch"`
	ProductionUseMainbranch  types.Bool   `tfsdk:"production_use_mainbranch"`
}

func (r *projectBranchingModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_branching_model"
}

func (r *projectBranchingModelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures the default development/production branch settings inherited by every repository in a Bitbucket Cloud project. There is exactly one of these per project; Bitbucket has no delete endpoint for it, so destroying this resource only removes it from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/project_key\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_key": schema.StringAttribute{
				Required:    true,
				Description: "Key of the project to configure.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"development_branch": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the development branch. Ignored if development_use_mainbranch is true.",
			},
			"development_use_mainbranch": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Use each repository's main branch as the development branch. Defaults to true.",
			},
			"production_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether a production branch is configured. Defaults to false.",
			},
			"production_branch": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the production branch. Ignored if production_use_mainbranch is true.",
			},
			"production_use_mainbranch": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Use each repository's main branch as the production branch. Defaults to false.",
			},
		},
	}
}

func (r *projectBranchingModelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectBranchingModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectBranchingModelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateProjectBranchingModelSettings(ctx, plan.ProjectKey.ValueString(), projectBranchingModelToAPI(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error configuring Bitbucket project branching model", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectBranchingModelModelFromAPI(r.client.Workspace(), plan.ProjectKey.ValueString(), updated))...)
}

func (r *projectBranchingModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectBranchingModelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetProjectBranchingModelSettings(ctx, state.ProjectKey.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket project branching model", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectBranchingModelModelFromAPI(r.client.Workspace(), state.ProjectKey.ValueString(), settings))...)
}

func (r *projectBranchingModelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectBranchingModelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateProjectBranchingModelSettings(ctx, plan.ProjectKey.ValueString(), projectBranchingModelToAPI(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error configuring Bitbucket project branching model", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectBranchingModelModelFromAPI(r.client.Workspace(), plan.ProjectKey.ValueString(), updated))...)
}

// Delete is a no-op: see the comment on branchingModelResource.Delete.
func (r *projectBranchingModelResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *projectBranchingModelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("project_key"), req, resp)
}

func projectBranchingModelToAPI(m *projectBranchingModelResourceModel) *bitbucket.BranchingModelSettings {
	return &bitbucket.BranchingModelSettings{
		Development: &bitbucket.BranchingModelBranch{
			Name:          m.DevelopmentBranch.ValueString(),
			UseMainbranch: m.DevelopmentUseMainbranch.ValueBool(),
		},
		Production: &bitbucket.BranchingModelProduction{
			Enabled:       m.ProductionEnabled.ValueBool(),
			Name:          m.ProductionBranch.ValueString(),
			UseMainbranch: m.ProductionUseMainbranch.ValueBool(),
		},
	}
}

func projectBranchingModelModelFromAPI(workspace, projectKey string, s *bitbucket.BranchingModelSettings) projectBranchingModelResourceModel {
	m := projectBranchingModelResourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s", workspace, projectKey)),
		ProjectKey: types.StringValue(projectKey),
	}
	if s.Development != nil {
		m.DevelopmentBranch = types.StringValue(s.Development.Name)
		m.DevelopmentUseMainbranch = types.BoolValue(s.Development.UseMainbranch)
	}
	if s.Production != nil {
		m.ProductionEnabled = types.BoolValue(s.Production.Enabled)
		m.ProductionBranch = types.StringValue(s.Production.Name)
		m.ProductionUseMainbranch = types.BoolValue(s.Production.UseMainbranch)
	}
	return m
}
