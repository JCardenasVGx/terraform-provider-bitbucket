package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &pipelineVariableResource{}
var _ resource.ResourceWithImportState = &pipelineVariableResource{}

func NewPipelineVariableResource() resource.Resource {
	return &pipelineVariableResource{}
}

type pipelineVariableResource struct {
	client *bitbucket.Client
}

type pipelineVariableResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepositorySlug types.String `tfsdk:"repository_slug"`
	VariableUUID   types.String `tfsdk:"variable_uuid"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	Secured        types.Bool   `tfsdk:"secured"`
}

func (r *pipelineVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_variable"
}

func (r *pipelineVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A repository-level Bitbucket Pipelines variable. When secured = true, Bitbucket never returns the value again after creation, so Terraform keeps trusting the value in your configuration/state instead of refreshing it from the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug/variable_uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository this variable belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"variable_uuid": schema.StringAttribute{
				Computed:    true,
				Description: "Bitbucket-assigned UUID of the variable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Required:    true,
				Description: "Variable name, as used in bitbucket-pipelines.yml.",
			},
			"value": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Variable value.",
			},
			"secured": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "If true, Bitbucket hides the value from logs and the API after creation.",
			},
		},
	}
}

func (r *pipelineVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pipelineVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pipelineVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateRepoPipelineVariable(ctx, plan.RepositorySlug.ValueString(), &bitbucket.PipelineVariable{
		Key:     plan.Key.ValueString(),
		Value:   plan.Value.ValueString(),
		Secured: plan.Secured.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket pipeline variable", err.Error())
		return
	}

	model := pipelineVariableModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), created)
	model.Value = plan.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *pipelineVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pipelineVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := r.client.GetRepoPipelineVariable(ctx, state.RepositorySlug.ValueString(), state.VariableUUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket pipeline variable", err.Error())
		return
	}

	model := pipelineVariableModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), v)
	if v.Secured {
		model.Value = state.Value
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *pipelineVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pipelineVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateRepoPipelineVariable(ctx, plan.RepositorySlug.ValueString(), plan.VariableUUID.ValueString(), &bitbucket.PipelineVariable{
		Key:     plan.Key.ValueString(),
		Value:   plan.Value.ValueString(),
		Secured: plan.Secured.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket pipeline variable", err.Error())
		return
	}

	model := pipelineVariableModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), updated)
	model.Value = plan.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *pipelineVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pipelineVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRepoPipelineVariable(ctx, state.RepositorySlug.ValueString(), state.VariableUUID.ValueString())
	if err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket pipeline variable", err.Error())
	}
}

// ImportState accepts "repository_slug/variable_uuid". The value cannot be
// recovered from Bitbucket, so it must be set via a subsequent config apply.
func (r *pipelineVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/variable_uuid\", got: %q", req.ID),
		)
		return
	}

	var state pipelineVariableResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.VariableUUID = types.StringValue(parts[1])
	state.Value = types.StringValue("")
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func pipelineVariableModelFromAPI(workspace, repoSlug string, v *bitbucket.PipelineVariable) pipelineVariableResourceModel {
	return pipelineVariableResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, v.UUID)),
		RepositorySlug: types.StringValue(repoSlug),
		VariableUUID:   types.StringValue(v.UUID),
		Key:            types.StringValue(v.Key),
		Value:          types.StringValue(v.Value),
		Secured:        types.BoolValue(v.Secured),
	}
}
