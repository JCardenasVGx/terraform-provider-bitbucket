package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &groupsDataSource{}

func NewGroupsDataSource() datasource.DataSource {
	return &groupsDataSource{}
}

type groupsDataSource struct {
	client *bitbucket.Client
}

type groupSummaryModel struct {
	Slug       types.String `tfsdk:"slug"`
	Name       types.String `tfsdk:"name"`
	Permission types.String `tfsdk:"permission"`
	AutoAdd    types.Bool   `tfsdk:"auto_add"`
}

type groupsDataSourceModel struct {
	ID     types.String        `tfsdk:"id"`
	Groups []groupSummaryModel `tfsdk:"groups"`
}

func (d *groupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_groups"
}

func (d *groupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every group in the workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier (the workspace slug).",
			},
			"groups": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every group in the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"slug":       schema.StringAttribute{Computed: true, Description: "Slug of the group."},
						"name":       schema.StringAttribute{Computed: true, Description: "Display name of the group."},
						"permission": schema.StringAttribute{Computed: true, Description: "Default repository permission granted to new members."},
						"auto_add":   schema.BoolAttribute{Computed: true, Description: "Whether new workspace members are automatically added to this group."},
					},
				},
			},
		},
	}
}

func (d *groupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	groups, err := d.client.ListGroups(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Bitbucket groups", err.Error())
		return
	}

	model := groupsDataSourceModel{
		ID: types.StringValue(d.client.Workspace()),
	}
	for _, g := range groups {
		model.Groups = append(model.Groups, groupSummaryModel{
			Slug:       types.StringValue(g.Slug),
			Name:       types.StringValue(g.Name),
			Permission: types.StringValue(g.Permission),
			AutoAdd:    types.BoolValue(g.AutoAdd),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
