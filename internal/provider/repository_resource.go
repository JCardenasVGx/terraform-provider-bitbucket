package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/bitbucket"
)

var _ resource.Resource = &repositoryResource{}
var _ resource.ResourceWithImportState = &repositoryResource{}

func NewRepositoryResource() resource.Resource {
	return &repositoryResource{}
}

type repositoryResource struct {
	client *bitbucket.Client
}

type repositoryResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Slug        types.String `tfsdk:"slug"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsPrivate   types.Bool   `tfsdk:"is_private"`
	ProjectKey  types.String `tfsdk:"project_key"`
	ForkPolicy  types.String `tfsdk:"fork_policy"`
	Language    types.String `tfsdk:"language"`
	HasIssues   types.Bool   `tfsdk:"has_issues"`
	HasWiki     types.Bool   `tfsdk:"has_wiki"`
	MainBranch  types.String `tfsdk:"main_branch"`
}

func (r *repositoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository"
}

func (r *repositoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Bitbucket Cloud Git repository.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Import/state identifier, formatted as \"workspace/slug\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"slug": schema.StringAttribute{
				Required:    true,
				Description: "Repository slug, used in the clone URL. Cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Repository display name. Defaults to the slug.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Repository description.",
			},
			"is_private": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the repository is private. Defaults to true.",
			},
			"project_key": schema.StringAttribute{
				Required:    true,
				Description: "Key of the Bitbucket project this repository belongs to.",
			},
			"fork_policy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "One of \"allow_forks\", \"no_public_forks\" or \"no_forks\". Defaults to \"no_forks\".",
			},
			"language": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Main programming language, as reported by Bitbucket.",
			},
			"has_issues": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the issue tracker is enabled. Defaults to false.",
			},
			"has_wiki": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the wiki is enabled. Defaults to false.",
			},
			"main_branch": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the repository's default branch.",
			},
		},
	}
}

func (r *repositoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*bitbucket.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *bitbucket.Client")
		return
	}
	r.client = client
}

func (r *repositoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan repositoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := repositoryToAPI(&plan)
	body.SCM = "git"

	created, err := r.client.CreateRepository(ctx, plan.Slug.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Bitbucket repository", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryModelFromAPI(r.client.Workspace(), created))...)
}

func (r *repositoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state repositoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.GetRepository(ctx, state.Slug.ValueString())
	if bitbucket.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Bitbucket repository", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryModelFromAPI(r.client.Workspace(), repo))...)
}

func (r *repositoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan repositoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateRepository(ctx, plan.Slug.ValueString(), repositoryToAPI(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating Bitbucket repository", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, repositoryModelFromAPI(r.client.Workspace(), updated))...)
}

func (r *repositoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state repositoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRepository(ctx, state.Slug.ValueString()); err != nil && !bitbucket.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Bitbucket repository", err.Error())
	}
}

func (r *repositoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("slug"), req, resp)
}

func repositoryToAPI(m *repositoryResourceModel) *bitbucket.Repository {
	isPrivate := true
	if !m.IsPrivate.IsNull() && !m.IsPrivate.IsUnknown() {
		isPrivate = m.IsPrivate.ValueBool()
	}
	hasIssues := m.HasIssues.ValueBool()
	hasWiki := m.HasWiki.ValueBool()

	r := &bitbucket.Repository{
		Name:        m.Name.ValueString(),
		Description: m.Description.ValueString(),
		IsPrivate:   &isPrivate,
		ForkPolicy:  m.ForkPolicy.ValueString(),
		HasIssues:   &hasIssues,
		HasWiki:     &hasWiki,
		Project:     &bitbucket.ProjectLink{Key: m.ProjectKey.ValueString()},
	}
	if mb := m.MainBranch.ValueString(); mb != "" {
		r.MainBranch = &bitbucket.MainBranchLink{Name: mb}
	}
	return r
}

func repositoryModelFromAPI(workspace string, r *bitbucket.Repository) repositoryResourceModel {
	m := repositoryResourceModel{
		ID:          types.StringValue(fmt.Sprintf("%s/%s", workspace, r.Slug)),
		Slug:        types.StringValue(r.Slug),
		Name:        types.StringValue(r.Name),
		Description: types.StringValue(r.Description),
		ForkPolicy:  types.StringValue(r.ForkPolicy),
		Language:    types.StringValue(r.Language),
	}
	if r.IsPrivate != nil {
		m.IsPrivate = types.BoolValue(*r.IsPrivate)
	}
	if r.HasIssues != nil {
		m.HasIssues = types.BoolValue(*r.HasIssues)
	}
	if r.HasWiki != nil {
		m.HasWiki = types.BoolValue(*r.HasWiki)
	}
	if r.Project != nil {
		m.ProjectKey = types.StringValue(r.Project.Key)
	}
	if r.MainBranch != nil {
		m.MainBranch = types.StringValue(r.MainBranch.Name)
	} else {
		m.MainBranch = types.StringValue("")
	}
	return m
}
