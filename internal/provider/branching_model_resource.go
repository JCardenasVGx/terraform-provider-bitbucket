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

var _ resource.Resource = &branchingModelResource{}
var _ resource.ResourceWithImportState = &branchingModelResource{}

func NewBranchingModelResource() resource.Resource {
	return &branchingModelResource{}
}

type branchingModelResource struct {
	client *bitbucket.Client
}

type branchingModelResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	RepositorySlug           types.String `tfsdk:"repository_slug"`
	DevelopmentBranch        types.String `tfsdk:"development_branch"`
	DevelopmentUseMainbranch types.Bool   `tfsdk:"development_use_mainbranch"`
	ProductionEnabled        types.Bool   `tfsdk:"production_enabled"`
	ProductionBranch         types.String `tfsdk:"production_branch"`
	ProductionUseMainbranch  types.Bool   `tfsdk:"production_use_mainbranch"`
}

func (r *branchingModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_branching_model"
}

func (r *branchingModelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures the development/production branches of a repository's branching model. There is exactly one of these per repository; Bitbucket has no delete endpoint for it, so destroying this resource only removes it from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository to configure.",
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
				Description: "Use the repository's main branch as the development branch. Defaults to true.",
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
				Description: "Use the repository's main branch as the production branch. Defaults to false.",
			},
		},
	}
}

func (r *branchingModelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *branchingModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan branchingModelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateBranchingModelSettings(ctx, plan.RepositorySlug.ValueString(), branchingModelToAPI(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error configuring Bitbucket branching model", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, branchingModelModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), updated))...)
}

func (r *branchingModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state branchingModelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetBranchingModelSettings(ctx, state.RepositorySlug.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket branching model", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, branchingModelModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), settings))...)
}

func (r *branchingModelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan branchingModelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateBranchingModelSettings(ctx, plan.RepositorySlug.ValueString(), branchingModelToAPI(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error configuring Bitbucket branching model", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, branchingModelModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), updated))...)
}

// Delete is a no-op: Bitbucket has no endpoint to delete branching model
// settings, only to overwrite them. Destroying this resource just stops
// Terraform from managing it.
func (r *branchingModelResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *branchingModelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("repository_slug"), req, resp)
}

func branchingModelToAPI(m *branchingModelResourceModel) *bitbucket.BranchingModelSettings {
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

func branchingModelModelFromAPI(workspace, repoSlug string, s *bitbucket.BranchingModelSettings) branchingModelResourceModel {
	m := branchingModelResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s", workspace, repoSlug)),
		RepositorySlug: types.StringValue(repoSlug),
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
