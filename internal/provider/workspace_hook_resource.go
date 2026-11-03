package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &workspaceHookResource{}
var _ resource.ResourceWithImportState = &workspaceHookResource{}

func NewWorkspaceHookResource() resource.Resource {
	return &workspaceHookResource{}
}

type workspaceHookResource struct {
	client *bitbucket.Client
}

type workspaceHookResourceModel struct {
	ID          types.String `tfsdk:"id"`
	UUID        types.String `tfsdk:"uuid"`
	URL         types.String `tfsdk:"url"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Events      types.List   `tfsdk:"events"`
}

func (r *workspaceHookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_hook"
}

func (r *workspaceHookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A webhook on a Bitbucket Cloud workspace, firing for events across every repository in it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/uuid\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
				Description: "Events that trigger this webhook, e.g. [\"repo:push\", \"pullrequest:created\"].",
			},
		},
	}
}

func (r *workspaceHookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workspaceHookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceHookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateWorkspaceHook(ctx, workspaceHookToAPI(ctx, &plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket workspace webhook", err.Error())
		return
	}

	model, diags := workspaceHookModelFromAPI(ctx, r.client.Workspace(), created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *workspaceHookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceHookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hook, err := r.client.GetWorkspaceHook(ctx, state.UUID.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket workspace webhook", err.Error())
		return
	}

	model, diags := workspaceHookModelFromAPI(ctx, r.client.Workspace(), hook)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *workspaceHookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceHookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateWorkspaceHook(ctx, plan.UUID.ValueString(), workspaceHookToAPI(ctx, &plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket workspace webhook", err.Error())
		return
	}

	model, diags := workspaceHookModelFromAPI(ctx, r.client.Workspace(), updated)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *workspaceHookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceHookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWorkspaceHook(ctx, state.UUID.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket workspace webhook", err.Error())
	}
}

func (r *workspaceHookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uuid"), req, resp)
}

func workspaceHookToAPI(ctx context.Context, m *workspaceHookResourceModel) *bitbucket.Hook {
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

func workspaceHookModelFromAPI(ctx context.Context, workspace string, h *bitbucket.Hook) (workspaceHookResourceModel, diag.Diagnostics) {
	m := workspaceHookResourceModel{
		ID:          types.StringValue(fmt.Sprintf("%s/%s", workspace, h.UUID)),
		UUID:        types.StringValue(h.UUID),
		URL:         types.StringValue(h.URL),
		Description: types.StringValue(h.Description),
		Active:      types.BoolValue(h.Active),
	}
	eventsList, diags := types.ListValueFrom(ctx, types.StringType, h.Events)
	m.Events = eventsList
	return m, diags
}
