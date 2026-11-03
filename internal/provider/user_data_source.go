package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &userDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

type userDataSource struct {
	client *bitbucket.Client
}

type userDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	SelectedUser types.String `tfsdk:"selected_user"`
	UUID         types.String `tfsdk:"uuid"`
	Username     types.String `tfsdk:"username"`
	AccountID    types.String `tfsdk:"account_id"`
	DisplayName  types.String `tfsdk:"display_name"`
	Nickname     types.String `tfsdk:"nickname"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a Bitbucket Cloud user, so you can resolve their UUID for use in other resources (e.g. bitbucket_branch_restriction's users, or bitbucket_repository_user_permission's user) instead of hardcoding it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier (the user's UUID).",
			},
			"selected_user": schema.StringAttribute{
				Required:    true,
				Description: "UUID, username or Atlassian account ID to look up.",
			},
			"uuid": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the user.",
			},
			"username": schema.StringAttribute{
				Computed:    true,
				Description: "Username, if the account still has one (new accounts are identified by account_id instead).",
			},
			"account_id": schema.StringAttribute{
				Computed:    true,
				Description: "Atlassian account ID of the user.",
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "Display name of the user.",
			},
			"nickname": schema.StringAttribute{
				Computed:    true,
				Description: "Nickname of the user.",
			},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := d.client.GetUser(ctx, cfg.SelectedUser.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket user", err.Error())
		return
	}

	model := userDataSourceModel{
		ID:           types.StringValue(user.UUID),
		SelectedUser: cfg.SelectedUser,
		UUID:         types.StringValue(user.UUID),
		Username:     types.StringValue(user.Username),
		AccountID:    types.StringValue(user.AccountID),
		DisplayName:  types.StringValue(user.DisplayName),
		Nickname:     types.StringValue(user.Nickname),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
