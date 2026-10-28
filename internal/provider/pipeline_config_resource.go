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

var _ resource.Resource = &pipelineConfigResource{}
var _ resource.ResourceWithImportState = &pipelineConfigResource{}

func NewPipelineConfigResource() resource.Resource {
	return &pipelineConfigResource{}
}

type pipelineConfigResource struct {
	client *bitbucket.Client
}

type pipelineConfigResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepositorySlug types.String `tfsdk:"repository_slug"`
	Enabled        types.Bool   `tfsdk:"enabled"`
}

func (r *pipelineConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_config"
}

func (r *pipelineConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables or disables Bitbucket Pipelines for a repository. There is exactly one of these per repository: destroying this resource disables Pipelines again, it does not delete the repository.",
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
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether Pipelines is enabled for the repository.",
			},
		},
	}
}

func (r *pipelineConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pipelineConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pipelineConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, err := r.client.UpdatePipelineConfig(ctx, plan.RepositorySlug.ValueString(), plan.Enabled.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Error configuring Bitbucket pipelines", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, pipelineConfigModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), cfg))...)
}

func (r *pipelineConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pipelineConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, err := r.client.GetPipelineConfig(ctx, state.RepositorySlug.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket pipelines config", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, pipelineConfigModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), cfg))...)
}

func (r *pipelineConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pipelineConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, err := r.client.UpdatePipelineConfig(ctx, plan.RepositorySlug.ValueString(), plan.Enabled.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Error configuring Bitbucket pipelines", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, pipelineConfigModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), cfg))...)
}

func (r *pipelineConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pipelineConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdatePipelineConfig(ctx, state.RepositorySlug.ValueString(), false); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error disabling Bitbucket pipelines", err.Error())
	}
}

func (r *pipelineConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("repository_slug"), req, resp)
}

func pipelineConfigModelFromAPI(workspace, repoSlug string, cfg *bitbucket.PipelinesConfig) pipelineConfigResourceModel {
	return pipelineConfigResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s", workspace, repoSlug)),
		RepositorySlug: types.StringValue(repoSlug),
		Enabled:        types.BoolValue(cfg.Enabled),
	}
}
