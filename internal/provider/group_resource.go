package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &groupResource{}
var _ resource.ResourceWithImportState = &groupResource{}

func NewGroupResource() resource.Resource {
	return &groupResource{}
}

type groupResource struct {
	client *bitbucket.Client
}

type groupResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Slug       types.String `tfsdk:"slug"`
	Permission types.String `tfsdk:"permission"`
	AutoAdd    types.Bool   `tfsdk:"auto_add"`
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A workspace group. Uses Bitbucket Cloud's legacy v1.0 API, since there is no v2.0 replacement for group management.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/slug\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the group.",
			},
			"slug": schema.StringAttribute{
				Computed:    true,
				Description: "Bitbucket-assigned slug of the group, derived from its name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"permission": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Default repository permission granted to new members: \"read\", \"write\" or \"admin\". Leave unset for no default permission.",
				Validators: []validator.String{
					stringvalidator.OneOf("read", "write", "admin"),
				},
			},
			"auto_add": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether new workspace members are automatically added to this group. Defaults to false.",
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateGroup(ctx, &bitbucket.Group{Name: plan.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket group", err.Error())
		return
	}

	// permission/auto_add aren't reliably accepted on create across
	// Bitbucket Cloud instances; set them explicitly as a follow-up update.
	if (!plan.Permission.IsNull() && !plan.Permission.IsUnknown() && plan.Permission.ValueString() != "") || (!plan.AutoAdd.IsNull() && !plan.AutoAdd.IsUnknown() && plan.AutoAdd.ValueBool()) {
		updated, err := r.client.UpdateGroup(ctx, created.Slug, &bitbucket.Group{
			Name:       plan.Name.ValueString(),
			Permission: plan.Permission.ValueString(),
			AutoAdd:    plan.AutoAdd.ValueBool(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error setting Bitbucket group permission/auto_add", err.Error())
			return
		}
		created = updated
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, groupModelFromAPI(r.client.Workspace(), created))...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.GetGroup(ctx, state.Slug.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket group", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, groupModelFromAPI(r.client.Workspace(), group))...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateGroup(ctx, plan.Slug.ValueString(), &bitbucket.Group{
		Name:       plan.Name.ValueString(),
		Permission: plan.Permission.ValueString(),
		AutoAdd:    plan.AutoAdd.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket group", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, groupModelFromAPI(r.client.Workspace(), updated))...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGroup(ctx, state.Slug.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket group", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("slug"), req, resp)
}

func groupModelFromAPI(workspace string, g *bitbucket.Group) groupResourceModel {
	return groupResourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s", workspace, g.Slug)),
		Name:       types.StringValue(g.Name),
		Slug:       types.StringValue(g.Slug),
		Permission: types.StringValue(g.Permission),
		AutoAdd:    types.BoolValue(g.AutoAdd),
	}
}
