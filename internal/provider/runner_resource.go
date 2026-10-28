package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &runnerResource{}
var _ resource.ResourceWithImportState = &runnerResource{}

func NewRunnerResource() resource.Resource {
	return &runnerResource{}
}

type runnerResource struct {
	client *bitbucket.Client
}

type runnerResourceModel struct {
	ID                types.String `tfsdk:"id"`
	RunnerUUID        types.String `tfsdk:"runner_uuid"`
	Name              types.String `tfsdk:"name"`
	Labels            types.List   `tfsdk:"labels"`
	Status            types.String `tfsdk:"status"`
	OauthClientID     types.String `tfsdk:"oauth_client_id"`
	OauthClientSecret types.String `tfsdk:"oauth_client_secret"`
}

func (r *runnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (r *runnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers a self-hosted Pipelines runner for a workspace. This only registers the runner in Bitbucket and returns its OAuth credentials; installing and running the actual runner daemon/container is done outside of Terraform (e.g. via the Docker image Bitbucket provides).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/runner_uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_uuid": schema.StringAttribute{
				Computed:    true,
				Description: "Bitbucket-assigned UUID of the runner.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the runner.",
			},
			"labels": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Labels used to target this runner from bitbucket-pipelines.yml via runs-on.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current runner status as reported by Bitbucket (e.g. ONLINE, OFFLINE).",
			},
			"oauth_client_id": schema.StringAttribute{
				Computed:    true,
				Description: "OAuth client ID issued for the runner daemon to authenticate with.",
			},
			"oauth_client_secret": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "OAuth client secret issued for the runner daemon. Bitbucket only returns this once, at creation time; it is preserved in state afterwards and cannot be recovered if lost (re-create the resource to get a new one).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *runnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *runnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labels := stringListToSlice(ctx, plan.Labels)

	created, err := r.client.CreateWorkspaceRunner(ctx, plan.Name.ValueString(), labels)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket runner", err.Error())
		return
	}

	model, diags := runnerModelFromAPI(ctx, r.client.Workspace(), created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *runnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runner, err := r.client.GetWorkspaceRunner(ctx, state.RunnerUUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket runner", err.Error())
		return
	}

	model, diags := runnerModelFromAPI(ctx, r.client.Workspace(), runner)
	resp.Diagnostics.Append(diags...)
	model.OauthClientSecret = state.OauthClientSecret
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *runnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labels := stringListToSlice(ctx, plan.Labels)

	updated, err := r.client.UpdateWorkspaceRunner(ctx, plan.RunnerUUID.ValueString(), plan.Name.ValueString(), labels)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket runner", err.Error())
		return
	}

	model, diags := runnerModelFromAPI(ctx, r.client.Workspace(), updated)
	resp.Diagnostics.Append(diags...)
	model.OauthClientSecret = plan.OauthClientSecret
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *runnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWorkspaceRunner(ctx, state.RunnerUUID.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket runner", err.Error())
	}
}

func (r *runnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("runner_uuid"), req, resp)
}

func stringListToSlice(ctx context.Context, l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	l.ElementsAs(ctx, &out, false)
	return out
}

func runnerModelFromAPI(ctx context.Context, workspace string, rn *bitbucket.Runner) (runnerResourceModel, diag.Diagnostics) {
	m := runnerResourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s", workspace, rn.UUID)),
		RunnerUUID: types.StringValue(rn.UUID),
		Name:       types.StringValue(rn.Name),
	}

	labelsList, diags := types.ListValueFrom(ctx, types.StringType, rn.Labels)
	m.Labels = labelsList

	if rn.State != nil {
		m.Status = types.StringValue(rn.State.Status)
	} else {
		m.Status = types.StringValue("")
	}

	if rn.OauthClient != nil {
		m.OauthClientID = types.StringValue(rn.OauthClient.ID)
		m.OauthClientSecret = types.StringValue(rn.OauthClient.Secret)
	} else {
		m.OauthClientID = types.StringValue("")
		m.OauthClientSecret = types.StringValue("")
	}

	return m, diags
}
