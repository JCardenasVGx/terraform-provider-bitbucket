package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &groupMembershipResource{}
var _ resource.ResourceWithImportState = &groupMembershipResource{}

func NewGroupMembershipResource() resource.Resource {
	return &groupMembershipResource{}
}

type groupMembershipResource struct {
	client *bitbucket.Client
}

type groupMembershipResourceModel struct {
	ID        types.String `tfsdk:"id"`
	GroupSlug types.String `tfsdk:"group_slug"`
	User      types.String `tfsdk:"user"`
}

func (r *groupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

func (r *groupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adds a user to a workspace group. One resource per user; add one of these per member you want in the group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/group_slug/user\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the group to add the user to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Required:    true,
				Description: "UUID or username of the user to add to the group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *groupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AddGroupMember(ctx, plan.GroupSlug.ValueString(), plan.User.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error adding Bitbucket group member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, groupMembershipModel(r.client.Workspace(), plan.GroupSlug.ValueString(), plan.User.ValueString()))...)
}

func (r *groupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	isMember, err := r.client.IsGroupMember(ctx, state.GroupSlug.ValueString(), state.User.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket group membership", err.Error())
		return
	}
	if !isMember {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, groupMembershipModel(r.client.Workspace(), state.GroupSlug.ValueString(), state.User.ValueString()))...)
}

func (r *groupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveGroupMember(ctx, state.GroupSlug.ValueString(), state.User.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing Bitbucket group member", err.Error())
	}
}

// ImportState accepts "group_slug/user".
func (r *groupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier \"group_slug/user\", got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, groupMembershipResourceModel{
		GroupSlug: types.StringValue(parts[0]),
		User:      types.StringValue(parts[1]),
	})...)
}

func groupMembershipModel(workspace, groupSlug, user string) groupMembershipResourceModel {
	return groupMembershipResourceModel{
		ID:        types.StringValue(fmt.Sprintf("%s/%s/%s", workspace, groupSlug, user)),
		GroupSlug: types.StringValue(groupSlug),
		User:      types.StringValue(user),
	}
}
