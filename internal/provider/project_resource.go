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

var _ resource.Resource = &projectResource{}
var _ resource.ResourceWithImportState = &projectResource{}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectResource struct {
	client *bitbucket.Client
}

type projectResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Key         types.String `tfsdk:"key"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsPrivate   types.Bool   `tfsdk:"is_private"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Bitbucket Cloud project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Import/state identifier, formatted as \"workspace/key\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Required:    true,
				Description: "Project key, uppercase alphanumeric. Cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Project display name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Project description.",
			},
			"is_private": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the project is private. Defaults to true.",
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	isPrivate := true
	if !plan.IsPrivate.IsNull() && !plan.IsPrivate.IsUnknown() {
		isPrivate = plan.IsPrivate.ValueBool()
	}

	created, err := r.client.CreateProject(ctx, &bitbucket.Project{
		Key:         plan.Key.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		IsPrivate:   isPrivate,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectModelFromAPI(r.client.Workspace(), created))...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.GetProject(ctx, state.Key.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectModelFromAPI(r.client.Workspace(), project))...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateProject(ctx, plan.Key.ValueString(), &bitbucket.Project{
		Key:         plan.Key.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		IsPrivate:   plan.IsPrivate.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectModelFromAPI(r.client.Workspace(), updated))...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteProject(ctx, state.Key.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID is just the project key; the workspace comes from the provider config.
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}

func projectModelFromAPI(workspace string, p *bitbucket.Project) projectResourceModel {
	return projectResourceModel{
		ID:          types.StringValue(fmt.Sprintf("%s/%s", workspace, p.Key)),
		Key:         types.StringValue(p.Key),
		Name:        types.StringValue(p.Name),
		Description: types.StringValue(p.Description),
		IsPrivate:   types.BoolValue(p.IsPrivate),
	}
}
