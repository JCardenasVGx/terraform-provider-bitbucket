package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &workspaceMembersDataSource{}

func NewWorkspaceMembersDataSource() datasource.DataSource {
	return &workspaceMembersDataSource{}
}

type workspaceMembersDataSource struct {
	client *bitbucket.Client
}

type workspaceMemberSummaryModel struct {
	UUID        types.String `tfsdk:"uuid"`
	Username    types.String `tfsdk:"username"`
	AccountID   types.String `tfsdk:"account_id"`
	DisplayName types.String `tfsdk:"display_name"`
	Nickname    types.String `tfsdk:"nickname"`
}

type workspaceMembersDataSourceModel struct {
	ID      types.String                  `tfsdk:"id"`
	Members []workspaceMemberSummaryModel `tfsdk:"members"`
}

func (d *workspaceMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_members"
}

func (d *workspaceMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every member of the workspace the provider is configured for.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier (the workspace slug).",
			},
			"members": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every member of the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid":         schema.StringAttribute{Computed: true, Description: "UUID of the member."},
						"username":     schema.StringAttribute{Computed: true, Description: "Username of the member, if the account still has one."},
						"account_id":   schema.StringAttribute{Computed: true, Description: "Atlassian account ID of the member."},
						"display_name": schema.StringAttribute{Computed: true, Description: "Display name of the member."},
						"nickname":     schema.StringAttribute{Computed: true, Description: "Nickname of the member."},
					},
				},
			},
		},
	}
}

func (d *workspaceMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workspaceMembersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	members, err := d.client.ListWorkspaceMembers(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Bitbucket workspace members", err.Error())
		return
	}

	model := workspaceMembersDataSourceModel{
		ID: types.StringValue(d.client.Workspace()),
	}
	for _, m := range members {
		model.Members = append(model.Members, workspaceMemberSummaryModel{
			UUID:        types.StringValue(m.UUID),
			Username:    types.StringValue(m.Username),
			AccountID:   types.StringValue(m.AccountID),
			DisplayName: types.StringValue(m.DisplayName),
			Nickname:    types.StringValue(m.Nickname),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
