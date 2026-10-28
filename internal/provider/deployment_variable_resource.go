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

var _ resource.Resource = &deploymentVariableResource{}
var _ resource.ResourceWithImportState = &deploymentVariableResource{}

func NewDeploymentVariableResource() resource.Resource {
	return &deploymentVariableResource{}
}

type deploymentVariableResource struct {
	client *bitbucket.Client
}

type deploymentVariableResourceModel struct {
	ID              types.String `tfsdk:"id"`
	RepositorySlug  types.String `tfsdk:"repository_slug"`
	EnvironmentUUID types.String `tfsdk:"environment_uuid"`
	VariableUUID    types.String `tfsdk:"variable_uuid"`
	Key             types.String `tfsdk:"key"`
	Value           types.String `tfsdk:"value"`
	Secured         types.Bool   `tfsdk:"secured"`
}

func (r *deploymentVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_variable"
}

func (r *deploymentVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A variable scoped to one Bitbucket Pipelines deployment environment. When secured = true, Bitbucket never returns the value again after creation, so Terraform trusts the value in your configuration/state instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug/environment_uuid/variable_uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository the environment belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_uuid": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the deployment environment this variable belongs to.",
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

func (r *deploymentVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *deploymentVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateDeploymentVariable(ctx, plan.RepositorySlug.ValueString(), plan.EnvironmentUUID.ValueString(), &bitbucket.DeploymentVariable{
		Key:     plan.Key.ValueString(),
		Value:   plan.Value.ValueString(),
		Secured: plan.Secured.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket deployment variable", err.Error())
		return
	}

	model := deploymentVariableModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.EnvironmentUUID.ValueString(), created)
	model.Value = plan.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *deploymentVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	v, err := r.client.GetDeploymentVariable(ctx, state.RepositorySlug.ValueString(), state.EnvironmentUUID.ValueString(), state.VariableUUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket deployment variable", err.Error())
		return
	}

	model := deploymentVariableModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), state.EnvironmentUUID.ValueString(), v)
	if v.Secured {
		model.Value = state.Value
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *deploymentVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan deploymentVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateDeploymentVariable(ctx, plan.RepositorySlug.ValueString(), plan.EnvironmentUUID.ValueString(), plan.VariableUUID.ValueString(), &bitbucket.DeploymentVariable{
		Key:     plan.Key.ValueString(),
		Value:   plan.Value.ValueString(),
		Secured: plan.Secured.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket deployment variable", err.Error())
		return
	}

	model := deploymentVariableModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), plan.EnvironmentUUID.ValueString(), updated)
	model.Value = plan.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *deploymentVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteDeploymentVariable(ctx, state.RepositorySlug.ValueString(), state.EnvironmentUUID.ValueString(), state.VariableUUID.ValueString())
	if err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket deployment variable", err.Error())
	}
}

// ImportState accepts "repository_slug/environment_uuid/variable_uuid".
func (r *deploymentVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/environment_uuid/variable_uuid\", got: %q", req.ID),
		)
		return
	}

	var state deploymentVariableResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.EnvironmentUUID = types.StringValue(parts[1])
	state.VariableUUID = types.StringValue(parts[2])
	state.Value = types.StringValue("")
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func deploymentVariableModelFromAPI(workspace, repoSlug, envUUID string, v *bitbucket.DeploymentVariable) deploymentVariableResourceModel {
	return deploymentVariableResourceModel{
		ID:              types.StringValue(fmt.Sprintf("%s/%s/%s/%s", workspace, repoSlug, envUUID, v.UUID)),
		RepositorySlug:  types.StringValue(repoSlug),
		EnvironmentUUID: types.StringValue(envUUID),
		VariableUUID:    types.StringValue(v.UUID),
		Key:             types.StringValue(v.Key),
		Value:           types.StringValue(v.Value),
		Secured:         types.BoolValue(v.Secured),
	}
}
