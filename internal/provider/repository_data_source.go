package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ datasource.DataSource = &repositoryDataSource{}

func NewRepositoryDataSource() datasource.DataSource {
	return &repositoryDataSource{}
}

type repositoryDataSource struct {
	client *bitbucket.Client
}

func (d *repositoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository"
}

func (d *repositoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Bitbucket Cloud repository by slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier, formatted as \"workspace/slug\".",
			},
			"slug": schema.StringAttribute{
				Required:    true,
				Description: "Repository slug.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Repository display name.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Repository description.",
			},
			"is_private": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the repository is private.",
			},
			"project_key": schema.StringAttribute{
				Computed:    true,
				Description: "Key of the Bitbucket project this repository belongs to.",
			},
			"fork_policy": schema.StringAttribute{
				Computed:    true,
				Description: "Fork policy.",
			},
			"language": schema.StringAttribute{
				Computed:    true,
				Description: "Main programming language, as reported by Bitbucket.",
			},
			"has_issues": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the issue tracker is enabled.",
			},
			"has_wiki": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the wiki is enabled.",
			},
			"main_branch": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the repository's default branch.",
			},
		},
	}
}

func (d *repositoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *repositoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg repositoryResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := d.client.GetRepository(ctx, cfg.Slug.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket repository", err.Error())
		return
	}

	model := repositoryModelFromAPI(d.client.Workspace(), repo)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
