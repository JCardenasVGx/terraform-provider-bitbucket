package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &hookResource{}
var _ resource.ResourceWithImportState = &hookResource{}

func NewHookResource() resource.Resource {
	return &hookResource{}
}

type hookResource struct {
	client *bitbucket.Client
}

type hookResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepositorySlug types.String `tfsdk:"repository_slug"`
	UUID           types.String `tfsdk:"uuid"`
	URL            types.String `tfsdk:"url"`
	Description    types.String `tfsdk:"description"`
	Active         types.Bool   `tfsdk:"active"`
	Events         types.List   `tfsdk:"events"`
}

func (r *hookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hook"
}

func (r *hookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A webhook on a Bitbucket Cloud repository.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Import/state identifier, formatted as \"workspace/repository_slug/uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repository_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the repository this webhook is attached to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"uuid": schema.StringAttribute{
				Computed:    true,
				Description: "Bitbucket-assigned UUID of the webhook.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"url": schema.StringAttribute{
				Required:    true,
				Description: "URL Bitbucket sends the webhook payload to.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the webhook.",
			},
			"active": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the webhook is active. Defaults to true.",
			},
			"events": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Events that trigger this webhook, e.g. [\"repo:push\", \"pullrequest:created\"]. See Bitbucket's webhook event payload docs for the full list.",
			},
		},
	}
}

func (r *hookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *hookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := hookToAPI(ctx, &plan)
	created, err := r.client.CreateHook(ctx, plan.RepositorySlug.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket webhook", err.Error())
		return
	}

	model, diags := hookModelFromAPI(ctx, r.client.Workspace(), plan.RepositorySlug.ValueString(), created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *hookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hook, err := r.client.GetHook(ctx, state.RepositorySlug.ValueString(), state.UUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket webhook", err.Error())
		return
	}

	model, diags := hookModelFromAPI(ctx, r.client.Workspace(), state.RepositorySlug.ValueString(), hook)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *hookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := hookToAPI(ctx, &plan)
	updated, err := r.client.UpdateHook(ctx, plan.RepositorySlug.ValueString(), plan.UUID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket webhook", err.Error())
		return
	}

	model, diags := hookModelFromAPI(ctx, r.client.Workspace(), plan.RepositorySlug.ValueString(), updated)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *hookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteHook(ctx, state.RepositorySlug.ValueString(), state.UUID.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket webhook", err.Error())
	}
}

// ImportState accepts "repository_slug/uuid".
func (r *hookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"repository_slug/uuid\", got: %q", req.ID),
		)
		return
	}
	var state hookResourceModel
	state.RepositorySlug = types.StringValue(parts[0])
	state.UUID = types.StringValue(parts[1])
	state.Events = types.ListNull(types.StringType)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func hookToAPI(ctx context.Context, m *hookResourceModel) *bitbucket.Hook {
	active := true
	if !m.Active.IsNull() && !m.Active.IsUnknown() {
		active = m.Active.ValueBool()
	}
	return &bitbucket.Hook{
		URL:         m.URL.ValueString(),
		Description: m.Description.ValueString(),
		Active:      active,
		Events:      stringListToSlice(ctx, m.Events),
	}
}

func hookModelFromAPI(ctx context.Context, workspace, repoSlug string, h *bitbucket.Hook) (hookResourceModel, diag.Diagnostics) {
	m := hookResourceModel{
		ID:             types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, repoSlug, h.UUID)),
		RepositorySlug: types.StringValue(repoSlug),
		UUID:           types.StringValue(h.UUID),
		URL:            types.StringValue(h.URL),
		Description:    types.StringValue(h.Description),
		Active:         types.BoolValue(h.Active),
	}
	eventsList, diags := types.ListValueFrom(ctx, types.StringType, h.Events)
	m.Events = eventsList
	return m, diags
}
