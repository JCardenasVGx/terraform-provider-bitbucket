package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &repositoryRunnerResource{}
var _ resource.ResourceWithImportState = &repositoryRunnerResource{}

func NewRepositoryRunnerResource() resource.Resource {
	return &repositoryRunnerResource{}
}

type repositoryRunnerResource struct {
	client *bitbucket.Client
}

type repositoryRunnerResourceModel struct {
	ID                types.String `tfsdk:"id"`
	RepositorySlug    types.String `tfsdk:"repository_slug"`
	RunnerUUID        types.String `tfsdk:"runner_uuid"`
	Name              types.String `tfsdk:"name"`
	Labels            types.List   `tfsdk:"labels"`
	Status            types.String `tfsdk:"status"`
	OauthClientID     types.String `tfsdk:"oauth_client_id"`
	OauthClientSecret types.String `tfsdk:"oauth_client_secret"`
}

func (r *repositoryRunnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_runner"
}

func (r *repositoryRunnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers a self-hosted Pipelines runner for a single repository. This only registers the runner in Bitbucket and returns its OAuth credentials; installing and running the actual runner daemon/container is done outside of Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug/runner_uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository this runner belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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

func (r *repositoryRunnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *repositoryRunnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan repositoryRunnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labels := stringListToSlice(ctx, plan.Labels)

	created, err := r.client.CreateRepositoryRunner(ctx, plan.RepositorySlug.ValueString(), plan.Name.ValueString(), labels)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket repository runner", err.Error())
		return
	}

	model, diags := repositoryRunnerModelFromAPI(ctx, r.client.Workspace(), plan.RepositorySlug.ValueString(), created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *repositoryRunnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state repositoryRunnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runner, err := r.client.GetRepositoryRunner(ctx, state.RepositorySlug.ValueString(), state.RunnerUUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket repository runner", err.Error())
		return
	}

	model, diags := repositoryRunnerModelFromAPI(ctx, r.client.Workspace(), state.RepositorySlug.ValueString(), runner)
	resp.Diagnostics.Append(diags...)
	model.OauthClientSecret = state.OauthClientSecret
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *repositoryRunnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan repositoryRunnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labels := stringListToSlice(ctx, plan.Labels)

	updated, err := r.client.UpdateRepositoryRunner(ctx, plan.RepositorySlug.ValueString(), plan.RunnerUUID.ValueString(), plan.Name.ValueString(), labels)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket repository runner", err.Error())
		return
	}

	model, diags := repositoryRunnerModelFromAPI(ctx, r.client.Workspace(), plan.RepositorySlug.ValueString(), updated)
	resp.Diagnostics.Append(diags...)
	model.OauthClientSecret = plan.OauthClientSecret
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *repositoryRunnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state repositoryRunnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRepositoryRunner(ctx, state.RepositorySlug.ValueString(), state.RunnerUUID.ValueString())
	if err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket repository runner", err.Error())
	}
}

// ImportState accepts "repository_slug/runner_uuid". The OAuth secret cannot
// be recovered from Bitbucket after creation, so it will be empty post-import.
func (r *repositoryRunnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/runner_uuid\", got: %q", req.ID),
		)
		return
	}

	var state repositoryRunnerResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.RunnerUUID = types.StringValue(parts[1])
	state.Name = types.StringNull()
	state.Status = types.StringNull()
	state.OauthClientID = types.StringNull()
	state.OauthClientSecret = types.StringNull()
	state.Labels = types.ListNull(types.StringType)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func repositoryRunnerModelFromAPI(ctx context.Context, workspace, repoSlug string, rn *bitbucket.Runner) (repositoryRunnerResourceModel, diag.Diagnostics) {
	m := repositoryRunnerResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, rn.UUID)),
		RepositorySlug: types.StringValue(repoSlug),
		RunnerUUID:     types.StringValue(rn.UUID),
		Name:           types.StringValue(rn.Name),
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
