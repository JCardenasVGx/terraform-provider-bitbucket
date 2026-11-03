package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &currentUserDataSource{}

func NewCurrentUserDataSource() datasource.DataSource {
	return &currentUserDataSource{}
}

type currentUserDataSource struct {
	client *bitbucket.Client
}

type currentUserDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	UUID        types.String `tfsdk:"uuid"`
	Username    types.String `tfsdk:"username"`
	AccountID   types.String `tfsdk:"account_id"`
	DisplayName types.String `tfsdk:"display_name"`
	Nickname    types.String `tfsdk:"nickname"`
}

func (d *currentUserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_current_user"
}

func (d *currentUserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up the Bitbucket Cloud account authenticated by the provider's credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier (the user's UUID).",
			},
			"uuid": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the authenticated user.",
			},
			"username": schema.StringAttribute{
				Computed:    true,
				Description: "Username, if the account still has one.",
			},
			"account_id": schema.StringAttribute{
				Computed:    true,
				Description: "Atlassian account ID of the authenticated user.",
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "Display name of the authenticated user.",
			},
			"nickname": schema.StringAttribute{
				Computed:    true,
				Description: "Nickname of the authenticated user.",
			},
		},
	}
}

func (d *currentUserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *currentUserDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	user, err := d.client.GetCurrentUser(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the current Bitbucket user", err.Error())
		return
	}

	model := currentUserDataSourceModel{
		ID:          types.StringValue(user.UUID),
		UUID:        types.StringValue(user.UUID),
		Username:    types.StringValue(user.Username),
		AccountID:   types.StringValue(user.AccountID),
		DisplayName: types.StringValue(user.DisplayName),
		Nickname:    types.StringValue(user.Nickname),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
