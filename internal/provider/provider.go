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
	Token     types.String `tfsdk:"token"`
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
				Description: "Bitbucket app password or Atlassian API token. Required unless token is set. Can also be set via BITBUCKET_PASSWORD.",
				Optional:    true,
				Sensitive:   true,
			},
			"token": schema.StringAttribute{
				Description: "A Bitbucket Repository, Project or Workspace Access Token. Sent as \"Authorization: Bearer <token>\" instead of HTTP Basic Auth; takes precedence over username/password for every resource except bitbucket_group and bitbucket_group_membership, which use Bitbucket's legacy v1.0 API and don't accept Access Tokens at all — those two still need username/password even when token is set. Can also be set via BITBUCKET_TOKEN.",
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
	token := firstNonEmpty(cfg.Token.ValueString(), os.Getenv("BITBUCKET_TOKEN"))

	if workspace == "" {
		resp.Diagnostics.AddError(
			"Missing Bitbucket workspace",
			"Set the workspace attribute on the provider block or the BITBUCKET_WORKSPACE environment variable.",
		)
	}

	haveBasicAuth := username != "" && password != ""
	if token == "" && !haveBasicAuth {
		resp.Diagnostics.AddError(
			"Missing Bitbucket credentials",
			"Set either token, or both username and password, on the provider block (or via BITBUCKET_TOKEN / BITBUCKET_USERNAME+BITBUCKET_PASSWORD).",
		)
	}
	if token != "" && !haveBasicAuth {
		resp.Diagnostics.AddWarning(
			"bitbucket_group/bitbucket_group_membership need username+password",
			"Access Tokens (token) can't authenticate against Bitbucket's legacy v1.0 groups API. bitbucket_group, bitbucket_group_membership and the group/groups/group_members data sources will fail unless username and password are also set.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	client := bitbucket.NewClient(workspace, username, password, token)
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
		NewRepositoryGroupPermissionResource,
		NewRepositoryUserPermissionResource,
		NewProjectGroupPermissionResource,
		NewProjectUserPermissionResource,
		NewDefaultReviewerResource,
		NewProjectDefaultReviewerResource,
		NewGroupResource,
		NewGroupMembershipResource,
		NewHookResource,
		NewWorkspaceHookResource,
		NewBranchingModelResource,
		NewProjectBranchingModelResource,
	}
}

func (p *bitbucketProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewRepositoryDataSource,
		NewWorkspaceDataSource,
		NewUserDataSource,
		NewCurrentUserDataSource,
		NewGroupDataSource,
		NewGroupsDataSource,
		NewGroupMembersDataSource,
		NewWorkspaceMembersDataSource,
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
