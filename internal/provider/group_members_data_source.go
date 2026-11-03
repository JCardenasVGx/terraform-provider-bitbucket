package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &groupMembersDataSource{}

func NewGroupMembersDataSource() datasource.DataSource {
	return &groupMembersDataSource{}
}

type groupMembersDataSource struct {
	client *bitbucket.Client
}

type groupMemberSummaryModel struct {
	UUID        types.String `tfsdk:"uuid"`
	Username    types.String `tfsdk:"username"`
	DisplayName types.String `tfsdk:"display_name"`
}

type groupMembersDataSourceModel struct {
	ID        types.String              `tfsdk:"id"`
	GroupSlug types.String              `tfsdk:"group_slug"`
	Members   []groupMemberSummaryModel `tfsdk:"members"`
}

func (d *groupMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_members"
}

func (d *groupMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every member of a workspace group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/group_slug\".",
			},
			"group_slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the group to list members of.",
			},
			"members": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every member of the group.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid":         schema.StringAttribute{Computed: true, Description: "UUID of the member."},
						"username":     schema.StringAttribute{Computed: true, Description: "Username of the member, if the account still has one."},
						"display_name": schema.StringAttribute{Computed: true, Description: "Display name of the member."},
					},
				},
			},
		},
	}
}

func (d *groupMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*bitbucket.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *bitbucket.Client")
		return
	}
	d.client = client
}

func (d *groupMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg groupMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	members, err := d.client.ListGroupMembers(ctx, cfg.GroupSlug.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Bitbucket group members", err.Error())
		return
	}

	model := groupMembersDataSourceModel{
		ID:        types.StringValue(fmt.Sprintf("%s/%s", d.client.Workspace(), cfg.GroupSlug.ValueString())),
		GroupSlug: cfg.GroupSlug,
	}
	for _, m := range members {
		model.Members = append(model.Members, groupMemberSummaryModel{
			UUID:        types.StringValue(m.UUID),
			Username:    types.StringValue(m.Username),
			DisplayName: types.StringValue(m.DisplayName),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
