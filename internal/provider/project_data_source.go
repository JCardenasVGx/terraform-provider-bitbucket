package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &projectDataSource{}

func NewProjectDataSource() datasource.DataSource {
	return &projectDataSource{}
}

type projectDataSource struct {
	client *bitbucket.Client
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Bitbucket Cloud project by key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/key\".",
			},
			"key": schema.StringAttribute{
				Required:    true,
				Description: "Project key.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Project display name.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Project description.",
			},
			"is_private": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the project is private.",
			},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg projectResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := d.client.GetProject(ctx, cfg.Key.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket project", err.Error())
		return
	}

	model := projectModelFromAPI(d.client.Workspace(), project)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
