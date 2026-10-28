package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &deploymentEnvironmentResource{}
var _ resource.ResourceWithImportState = &deploymentEnvironmentResource{}

func NewDeploymentEnvironmentResource() resource.Resource {
	return &deploymentEnvironmentResource{}
}

type deploymentEnvironmentResource struct {
	client *bitbucket.Client
}

type deploymentEnvironmentResourceModel struct {
	ID              types.String `tfsdk:"id"`
	RepositorySlug  types.String `tfsdk:"repository_slug"`
	EnvironmentUUID types.String `tfsdk:"environment_uuid"`
	Name            types.String `tfsdk:"name"`
	EnvironmentType types.String `tfsdk:"environment_type"`
	AdminOnly       types.Bool   `tfsdk:"admin_only"`
}

func (r *deploymentEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_environment"
}

func (r *deploymentEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Bitbucket Pipelines deployment environment (e.g. Staging, Production) on a repository.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/repository_slug/environment_uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository this environment belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_uuid": schema.StringAttribute{
				Computed:    true,
				Description: "Bitbucket-assigned UUID of the environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Environment display name.",
			},
			"environment_type": schema.StringAttribute{
				Required:    true,
				Description: "One of \"Test\", \"Staging\" or \"Production\". Cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("Test", "Staging", "Production"),
				},
			},
			"admin_only": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Restrict deployments to this environment to workspace admins.",
			},
		},
	}
}

func (r *deploymentEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *deploymentEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateEnvironment(ctx, plan.RepositorySlug.ValueString(), &bitbucket.DeploymentEnvironment{
		Name:            plan.Name.ValueString(),
		EnvironmentType: &bitbucket.EnvironmentType{Name: plan.EnvironmentType.ValueString()},
		Restrictions:    &bitbucket.EnvironmentRestrictions{AdminOnly: plan.AdminOnly.ValueBool()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket deployment environment", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, deploymentEnvironmentModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), created))...)
}

func (r *deploymentEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.RepositorySlug.ValueString(), state.EnvironmentUUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket deployment environment", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, deploymentEnvironmentModelFromAPI(r.client.Workspace(), state.RepositorySlug.ValueString(), env))...)
}

func (r *deploymentEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan deploymentEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateEnvironment(ctx, plan.RepositorySlug.ValueString(), plan.EnvironmentUUID.ValueString(), plan.Name.ValueString(), &bitbucket.EnvironmentRestrictions{AdminOnly: plan.AdminOnly.ValueBool()})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket deployment environment", err.Error())
		return
	}
	// The "changes" endpoint doesn't echo environment_type back reliably; keep it from the plan.
	if updated.EnvironmentType == nil {
		updated.EnvironmentType = &bitbucket.EnvironmentType{Name: plan.EnvironmentType.ValueString()}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, deploymentEnvironmentModelFromAPI(r.client.Workspace(), plan.RepositorySlug.ValueString(), updated))...)
}

func (r *deploymentEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteEnvironment(ctx, state.RepositorySlug.ValueString(), state.EnvironmentUUID.ValueString())
	if err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket deployment environment", err.Error())
	}
}

// ImportState accepts "repository_slug/environment_uuid".
func (r *deploymentEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/environment_uuid\", got: %q", req.ID),
		)
		return
	}

	var state deploymentEnvironmentResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.EnvironmentUUID = types.StringValue(parts[1])
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func deploymentEnvironmentModelFromAPI(workspace, repoSlug string, env *bitbucket.DeploymentEnvironment) deploymentEnvironmentResourceModel {
	m := deploymentEnvironmentResourceModel{
		ID:              types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, env.UUID)),
		RepositorySlug:  types.StringValue(repoSlug),
		EnvironmentUUID: types.StringValue(env.UUID),
		Name:            types.StringValue(env.Name),
	}
	if env.EnvironmentType != nil {
		m.EnvironmentType = types.StringValue(env.EnvironmentType.Name)
	} else {
		m.EnvironmentType = types.StringValue("")
	}
	if env.Restrictions != nil {
		m.AdminOnly = types.BoolValue(env.Restrictions.AdminOnly)
	} else {
		m.AdminOnly = types.BoolValue(false)
	}
	return m
}
