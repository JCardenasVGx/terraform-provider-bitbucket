package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &workspacePipelineVariableResource{}
var _ resource.ResourceWithImportState = &workspacePipelineVariableResource{}

func NewWorkspacePipelineVariableResource() resource.Resource {
	return &workspacePipelineVariableResource{}
}

type workspacePipelineVariableResource struct {
	client *bitbucket.Client
}

type workspacePipelineVariableResourceModel struct {
	ID           types.String `tfsdk:"id"`
	VariableUUID types.String `tfsdk:"variable_uuid"`
	Key          types.String `tfsdk:"key"`
	Value        types.String `tfsdk:"value"`
	Secured      types.Bool   `tfsdk:"secured"`
}

func (r *workspacePipelineVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_pipeline_variable"
}

func (r *workspacePipelineVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A workspace-level Bitbucket Pipelines variable, shared across every repository in the workspace. When secured = true, Bitbucket never returns the value again after creation, so Terraform trusts the value in your configuration/state instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/variable_uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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

func (r *workspacePipelineVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workspacePipelineVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspacePipelineVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateWorkspacePipelineVariable(ctx, &bitbucket.PipelineVariable{
		Key:     plan.Key.ValueString(),
		Value:   plan.Value.ValueString(),
		Secured: plan.Secured.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket workspace pipeline variable", err.Error())
		return
	}

	model := workspacePipelineVariableModelFromAPI(r.client.Workspace(), created)
	model.Value = plan.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *workspacePipelineVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspacePipelineVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := r.client.GetWorkspacePipelineVariable(ctx, state.VariableUUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket workspace pipeline variable", err.Error())
		return
	}

	model := workspacePipelineVariableModelFromAPI(r.client.Workspace(), v)
	if v.Secured {
		model.Value = state.Value
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *workspacePipelineVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspacePipelineVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateWorkspacePipelineVariable(ctx, plan.VariableUUID.ValueString(), &bitbucket.PipelineVariable{
		Key:     plan.Key.ValueString(),
		Value:   plan.Value.ValueString(),
		Secured: plan.Secured.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket workspace pipeline variable", err.Error())
		return
	}

	model := workspacePipelineVariableModelFromAPI(r.client.Workspace(), updated)
	model.Value = plan.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *workspacePipelineVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspacePipelineVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteWorkspacePipelineVariable(ctx, state.VariableUUID.ValueString())
	if err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket workspace pipeline variable", err.Error())
	}
}

func (r *workspacePipelineVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("variable_uuid"), req, resp)
}

func workspacePipelineVariableModelFromAPI(workspace string, v *bitbucket.PipelineVariable) workspacePipelineVariableResourceModel {
	return workspacePipelineVariableResourceModel{
		ID:           types.StringValue(fmt.Sprintf("%s/%s", workspace, v.UUID)),
		VariableUUID: types.StringValue(v.UUID),
		Key:          types.StringValue(v.Key),
		Value:        types.StringValue(v.Value),
		Secured:      types.BoolValue(v.Secured),
	}
}
