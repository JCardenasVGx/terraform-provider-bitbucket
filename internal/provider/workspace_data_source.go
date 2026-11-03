package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &workspaceDataSource{}

func NewWorkspaceDataSource() datasource.DataSource {
	return &workspaceDataSource{}
}

type workspaceDataSource struct {
	client *bitbucket.Client
}

type workspaceDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Slug      types.String `tfsdk:"slug"`
	Name      types.String `tfsdk:"name"`
	UUID      types.String `tfsdk:"uuid"`
	IsPrivate types.Bool   `tfsdk:"is_private"`
}

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up the Bitbucket Cloud workspace the provider is configured for.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier (the workspace slug).",
			},
			"slug": schema.StringAttribute{
				Computed:    true,
				Description: "Workspace slug.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Workspace display name.",
			},
			"uuid": schema.StringAttribute{
				Computed:    true,
				Description: "Bitbucket-assigned UUID of the workspace.",
			},
			"is_private": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the workspace is private.",
			},
		},
	}
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workspaceDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	ws, err := d.client.GetWorkspace(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket workspace", err.Error())
		return
	}

	model := workspaceDataSourceModel{
		ID:        types.StringValue(ws.Slug),
		Slug:      types.StringValue(ws.Slug),
		Name:      types.StringValue(ws.Name),
		UUID:      types.StringValue(ws.UUID),
		IsPrivate: types.BoolValue(ws.IsPrivate),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
