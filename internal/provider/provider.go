package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

// Ensure the implementation satisfies the expected interfaces.
var _ provider.Provider = &bitbucketProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &bitbucketProvider{version: version}
	}
}

type bitbucketProvider struct {
	version string
}

type bitbucketProviderModel struct {
	Workspace types.String `tfsdk:"workspace"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
}

func (p *bitbucketProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "bitbucket"
	resp.Version = p.version
}

func (p *bitbucketProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Bitbucket Cloud projects, repositories, branch restrictions, Pipelines configuration (pipeline config, variables, runners) and deployment environments.",
		Attributes: map[string]schema.Attribute{
			"workspace": schema.StringAttribute{
				Description: "Bitbucket Cloud workspace slug. Can also be set via the BITBUCKET_WORKSPACE environment variable.",
				Optional:    true,
			},
			"username": schema.StringAttribute{
				Description: "Bitbucket username (or Atlassian account email when using an API token). Can also be set via BITBUCKET_USERNAME.",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "Bitbucket app password or API token. Can also be set via BITBUCKET_PASSWORD.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *bitbucketProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg bitbucketProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace := firstNonEmpty(cfg.Workspace.ValueString(), os.Getenv("BITBUCKET_WORKSPACE"))
	username := firstNonEmpty(cfg.Username.ValueString(), os.Getenv("BITBUCKET_USERNAME"))
	password := firstNonEmpty(cfg.Password.ValueString(), os.Getenv("BITBUCKET_PASSWORD"))

	if workspace == "" {
		resp.Diagnostics.AddError(
			"Missing Bitbucket workspace",
			"Set the workspace attribute on the provider block or the BITBUCKET_WORKSPACE environment variable.",
		)
	}
	if username == "" {
		resp.Diagnostics.AddError(
			"Missing Bitbucket username",
			"Set the username attribute on the provider block or the BITBUCKET_USERNAME environment variable.",
		)
	}
	if password == "" {
		resp.Diagnostics.AddError(
			"Missing Bitbucket password",
			"Set the password attribute on the provider block or the BITBUCKET_PASSWORD environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	client := bitbucket.NewClient(workspace, username, password)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *bitbucketProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewRepositoryResource,
		NewBranchRestrictionResource,
		NewPipelineConfigResource,
		NewPipelineVariableResource,
		NewWorkspacePipelineVariableResource,
		NewRunnerResource,
		NewRepositoryRunnerResource,
		NewDeploymentEnvironmentResource,
		NewDeploymentVariableResource,
	}
}

func (p *bitbucketProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewRepositoryDataSource,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
