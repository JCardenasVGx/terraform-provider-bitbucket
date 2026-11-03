package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &groupDataSource{}

func NewGroupDataSource() datasource.DataSource {
	return &groupDataSource{}
}

type groupDataSource struct {
	client *bitbucket.Client
}

type groupDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Slug       types.String `tfsdk:"slug"`
	Name       types.String `tfsdk:"name"`
	Permission types.String `tfsdk:"permission"`
	AutoAdd    types.Bool   `tfsdk:"auto_add"`
}

func (d *groupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing workspace group by slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/slug\".",
			},
			"slug": schema.StringAttribute{
				Required:    true,
				Description: "Slug of the group to look up.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Display name of the group.",
			},
			"permission": schema.StringAttribute{
				Computed:    true,
				Description: "Default repository permission granted to new members.",
			},
			"auto_add": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether new workspace members are automatically added to this group.",
			},
		},
	}
}

func (d *groupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg groupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := d.client.GetGroup(ctx, cfg.Slug.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket group", err.Error())
		return
	}

	model := groupDataSourceModel{
		ID:         types.StringValue(fmt.Sprintf("%s/%s", d.client.Workspace(), group.Slug)),
		Slug:       types.StringValue(group.Slug),
		Name:       types.StringValue(group.Name),
		Permission: types.StringValue(group.Permission),
		AutoAdd:    types.BoolValue(group.AutoAdd),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
