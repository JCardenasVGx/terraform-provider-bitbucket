// Package bitbucket is a minimal REST client for the Bitbucket Cloud API,
// covering what the Terraform provider needs: projects, repositories,
// branch restrictions, repository/project permissions, default reviewers,
// webhooks, branching models and workspace groups (the latter via the
// legacy v1.0 API, which has no v2.0 replacement).
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	defaultBaseURL = "https://api.bitbucket.org/2.0"
	// defaultBaseURLV1 is the legacy v1.0 API. Bitbucket Cloud never shipped
	// a v2.0 replacement for workspace groups, so group management still
	// has to go through this deprecated-but-still-live version.
	defaultBaseURLV1 = "https://api.bitbucket.org/1.0"
)

// Client talks to the Bitbucket Cloud REST API on behalf of one workspace.
type Client struct {
	baseURL    string
	baseURLV1  string
	workspace  string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient builds a Bitbucket Cloud API client authenticated with HTTP
// Basic Auth (username + app password, or an Atlassian account email +
// API token).
func NewClient(workspace, username, password string) *Client {
	baseURL := defaultBaseURL
	baseURLV1 := defaultBaseURLV1
	if override := os.Getenv("BITBUCKET_BASE_URL"); override != "" {
		// Lets unit/acceptance tests point the client at a local mock server
		// instead of the real Bitbucket Cloud API. The mock server handles
		// both API versions on the same mux, so both bases point at it.
		baseURL = override
		baseURLV1 = override
	}
	return &Client{
		baseURL:    baseURL,
		baseURLV1:  baseURLV1,
		workspace:  workspace,
		username:   username,
		password:   password,
		httpClient: &http.Client{},
	}
}

// Workspace returns the workspace this client is scoped to.
func (c *Client) Workspace() string {
	return c.workspace
}

// APIError is returned when Bitbucket responds with a non-2xx status.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bitbucket api error: status %d: %s", e.StatusCode, e.Body)
}

// IsNotFound reports whether err is a 404 from the Bitbucket API.
func IsNotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.StatusCode == http.StatusNotFound
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	return c.doWithBase(ctx, c.baseURL, method, path, body, out)
}

// doV1 is like do but against the legacy v1.0 API (used only for workspace
// groups, which have no v2.0 equivalent).
func (c *Client) doV1(ctx context.Context, method, path string, body any, out any) error {
	return c.doWithBase(ctx, c.baseURLV1, method, path, body, out)
}

func (c *Client) doWithBase(ctx context.Context, base, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling bitbucket api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}
	}
	return nil
}

// --- Projects ---------------------------------------------------------

func (c *Client) GetProject(ctx context.Context, key string) (*Project, error) {
	var p Project
	path := fmt.Sprintf("/workspaces/%s/projects/%s", c.workspace, key)
	if err := c.do(ctx, http.MethodGet, path, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Client) CreateProject(ctx context.Context, p *Project) (*Project, error) {
	var out Project
	path := fmt.Sprintf("/workspaces/%s/projects", c.workspace)
	if err := c.do(ctx, http.MethodPost, path, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateProject(ctx context.Context, key string, p *Project) (*Project, error) {
	var out Project
	path := fmt.Sprintf("/workspaces/%s/projects/%s", c.workspace, key)
	if err := c.do(ctx, http.MethodPut, path, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteProject(ctx context.Context, key string) error {
	path := fmt.Sprintf("/workspaces/%s/projects/%s", c.workspace, key)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Repositories -------------------------------------------------------

func (c *Client) GetRepository(ctx context.Context, slug string) (*Repository, error) {
	var r Repository
	path := fmt.Sprintf("/repositories/%s/%s", c.workspace, slug)
	if err := c.do(ctx, http.MethodGet, path, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (c *Client) CreateRepository(ctx context.Context, slug string, r *Repository) (*Repository, error) {
	var out Repository
	path := fmt.Sprintf("/repositories/%s/%s", c.workspace, slug)
	if err := c.do(ctx, http.MethodPost, path, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRepository(ctx context.Context, slug string, r *Repository) (*Repository, error) {
	var out Repository
	path := fmt.Sprintf("/repositories/%s/%s", c.workspace, slug)
	if err := c.do(ctx, http.MethodPut, path, r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRepository(ctx context.Context, slug string) error {
	path := fmt.Sprintf("/repositories/%s/%s", c.workspace, slug)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Branch restrictions -------------------------------------------------

func (c *Client) GetBranchRestriction(ctx context.Context, repoSlug string, id int64) (*BranchRestriction, error) {
	var br BranchRestriction
	path := fmt.Sprintf("/repositories/%s/%s/branch-restrictions/%d", c.workspace, repoSlug, id)
	if err := c.do(ctx, http.MethodGet, path, nil, &br); err != nil {
		return nil, err
	}
	return &br, nil
}

func (c *Client) CreateBranchRestriction(ctx context.Context, repoSlug string, br *BranchRestriction) (*BranchRestriction, error) {
	var out BranchRestriction
	path := fmt.Sprintf("/repositories/%s/%s/branch-restrictions", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPost, path, br, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateBranchRestriction(ctx context.Context, repoSlug string, id int64, br *BranchRestriction) (*BranchRestriction, error) {
	var out BranchRestriction
	path := fmt.Sprintf("/repositories/%s/%s/branch-restrictions/%d", c.workspace, repoSlug, id)
	if err := c.do(ctx, http.MethodPut, path, br, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteBranchRestriction(ctx context.Context, repoSlug string, id int64) error {
	path := fmt.Sprintf("/repositories/%s/%s/branch-restrictions/%d", c.workspace, repoSlug, id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Repository permissions (groups/users) --------------------------------

func (c *Client) GetRepositoryGroupPermission(ctx context.Context, repoSlug, groupSlug string) (*RepositoryGroupPermission, error) {
	var out RepositoryGroupPermission
	path := fmt.Sprintf("/repositories/%s/%s/permissions-config/groups/%s", c.workspace, repoSlug, groupSlug)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetRepositoryGroupPermission(ctx context.Context, repoSlug, groupSlug, permission string) (*RepositoryGroupPermission, error) {
	var out RepositoryGroupPermission
	path := fmt.Sprintf("/repositories/%s/%s/permissions-config/groups/%s", c.workspace, repoSlug, groupSlug)
	// The group is already identified by the URL; sending it again in the
	// body would serialize as a non-empty object ({"slug":""}) since Go's
	// omitempty doesn't apply to non-pointer structs.
	body := &struct {
		Permission string `json:"permission"`
	}{Permission: permission}
	if err := c.do(ctx, http.MethodPut, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRepositoryGroupPermission(ctx context.Context, repoSlug, groupSlug string) error {
	path := fmt.Sprintf("/repositories/%s/%s/permissions-config/groups/%s", c.workspace, repoSlug, groupSlug)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) GetRepositoryUserPermission(ctx context.Context, repoSlug, selectedUser string) (*RepositoryUserPermission, error) {
	var out RepositoryUserPermission
	path := fmt.Sprintf("/repositories/%s/%s/permissions-config/users/%s", c.workspace, repoSlug, selectedUser)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetRepositoryUserPermission(ctx context.Context, repoSlug, selectedUser, permission string) (*RepositoryUserPermission, error) {
	var out RepositoryUserPermission
	path := fmt.Sprintf("/repositories/%s/%s/permissions-config/users/%s", c.workspace, repoSlug, selectedUser)
	// The user is already identified by the URL; see the comment in
	// SetRepositoryGroupPermission for why the body only carries permission.
	body := &struct {
		Permission string `json:"permission"`
	}{Permission: permission}
	if err := c.do(ctx, http.MethodPut, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRepositoryUserPermission(ctx context.Context, repoSlug, selectedUser string) error {
	path := fmt.Sprintf("/repositories/%s/%s/permissions-config/users/%s", c.workspace, repoSlug, selectedUser)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Pipelines config (enable/disable) -----------------------------------

func (c *Client) GetPipelineConfig(ctx context.Context, repoSlug string) (*PipelinesConfig, error) {
	var cfg PipelinesConfig
	path := fmt.Sprintf("/repositories/%s/%s/pipelines_config", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodGet, path, nil, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Client) UpdatePipelineConfig(ctx context.Context, repoSlug string, enabled bool) (*PipelinesConfig, error) {
	var out PipelinesConfig
	path := fmt.Sprintf("/repositories/%s/%s/pipelines_config", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPut, path, &PipelinesConfig{Enabled: enabled}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Repository pipeline variables ---------------------------------------

func (c *Client) CreateRepoPipelineVariable(ctx context.Context, repoSlug string, v *PipelineVariable) (*PipelineVariable, error) {
	var out PipelineVariable
	path := fmt.Sprintf("/repositories/%s/%s/pipelines_config/variables", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPost, path, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRepoPipelineVariable(ctx context.Context, repoSlug, uuid string) (*PipelineVariable, error) {
	var out PipelineVariable
	path := fmt.Sprintf("/repositories/%s/%s/pipelines_config/variables/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRepoPipelineVariable(ctx context.Context, repoSlug, uuid string, v *PipelineVariable) (*PipelineVariable, error) {
	var out PipelineVariable
	path := fmt.Sprintf("/repositories/%s/%s/pipelines_config/variables/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodPut, path, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRepoPipelineVariable(ctx context.Context, repoSlug, uuid string) error {
	path := fmt.Sprintf("/repositories/%s/%s/pipelines_config/variables/%s", c.workspace, repoSlug, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Workspace pipeline variables -----------------------------------------

func (c *Client) CreateWorkspacePipelineVariable(ctx context.Context, v *PipelineVariable) (*PipelineVariable, error) {
	var out PipelineVariable
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/variables", c.workspace)
	if err := c.do(ctx, http.MethodPost, path, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetWorkspacePipelineVariable(ctx context.Context, uuid string) (*PipelineVariable, error) {
	var out PipelineVariable
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/variables/%s", c.workspace, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateWorkspacePipelineVariable(ctx context.Context, uuid string, v *PipelineVariable) (*PipelineVariable, error) {
	var out PipelineVariable
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/variables/%s", c.workspace, uuid)
	if err := c.do(ctx, http.MethodPut, path, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteWorkspacePipelineVariable(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/variables/%s", c.workspace, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Runners ---------------------------------------------------------------

func (c *Client) CreateWorkspaceRunner(ctx context.Context, name string, labels []string) (*Runner, error) {
	var out Runner
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/runners", c.workspace)
	if err := c.do(ctx, http.MethodPost, path, &createRunnerRequest{Name: name, Labels: labels}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetWorkspaceRunner(ctx context.Context, uuid string) (*Runner, error) {
	var out Runner
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/runners/%s", c.workspace, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateWorkspaceRunner(ctx context.Context, uuid, name string, labels []string) (*Runner, error) {
	var out Runner
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/runners/%s", c.workspace, uuid)
	if err := c.do(ctx, http.MethodPut, path, &createRunnerRequest{Name: name, Labels: labels}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteWorkspaceRunner(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("/workspaces/%s/pipelines-config/runners/%s", c.workspace, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) CreateRepositoryRunner(ctx context.Context, repoSlug, name string, labels []string) (*Runner, error) {
	var out Runner
	path := fmt.Sprintf("/repositories/%s/%s/pipelines-config/runners", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPost, path, &createRunnerRequest{Name: name, Labels: labels}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRepositoryRunner(ctx context.Context, repoSlug, uuid string) (*Runner, error) {
	var out Runner
	path := fmt.Sprintf("/repositories/%s/%s/pipelines-config/runners/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRepositoryRunner(ctx context.Context, repoSlug, uuid, name string, labels []string) (*Runner, error) {
	var out Runner
	path := fmt.Sprintf("/repositories/%s/%s/pipelines-config/runners/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodPut, path, &createRunnerRequest{Name: name, Labels: labels}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRepositoryRunner(ctx context.Context, repoSlug, uuid string) error {
	path := fmt.Sprintf("/repositories/%s/%s/pipelines-config/runners/%s", c.workspace, repoSlug, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Deployment environments -----------------------------------------------

func (c *Client) CreateEnvironment(ctx context.Context, repoSlug string, env *DeploymentEnvironment) (*DeploymentEnvironment, error) {
	var out DeploymentEnvironment
	path := fmt.Sprintf("/repositories/%s/%s/environments/", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPost, path, env, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetEnvironment(ctx context.Context, repoSlug, uuid string) (*DeploymentEnvironment, error) {
	var out DeploymentEnvironment
	path := fmt.Sprintf("/repositories/%s/%s/environments/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, repoSlug, uuid, name string, restrictions *EnvironmentRestrictions) (*DeploymentEnvironment, error) {
	var out DeploymentEnvironment
	path := fmt.Sprintf("/repositories/%s/%s/environments/%s/changes/", c.workspace, repoSlug, uuid)
	body := &environmentChangeRequest{Change: &environmentChange{Name: name, Restrictions: restrictions}}
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, repoSlug, uuid string) error {
	path := fmt.Sprintf("/repositories/%s/%s/environments/%s", c.workspace, repoSlug, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Deployment variables ---------------------------------------------------

func (c *Client) CreateDeploymentVariable(ctx context.Context, repoSlug, envUUID string, v *DeploymentVariable) (*DeploymentVariable, error) {
	var out DeploymentVariable
	path := fmt.Sprintf("/repositories/%s/%s/deployments_config/environments/%s/variables", c.workspace, repoSlug, envUUID)
	if err := c.do(ctx, http.MethodPost, path, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDeploymentVariable(ctx context.Context, repoSlug, envUUID, varUUID string, v *DeploymentVariable) (*DeploymentVariable, error) {
	var out DeploymentVariable
	path := fmt.Sprintf("/repositories/%s/%s/deployments_config/environments/%s/variables/%s", c.workspace, repoSlug, envUUID, varUUID)
	if err := c.do(ctx, http.MethodPut, path, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDeploymentVariable(ctx context.Context, repoSlug, envUUID, varUUID string) error {
	path := fmt.Sprintf("/repositories/%s/%s/deployments_config/environments/%s/variables/%s", c.workspace, repoSlug, envUUID, varUUID)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// GetDeploymentVariable finds one variable by listing, since Bitbucket has
// no get-by-id endpoint for deployment variables.
func (c *Client) GetDeploymentVariable(ctx context.Context, repoSlug, envUUID, varUUID string) (*DeploymentVariable, error) {
	path := fmt.Sprintf("/repositories/%s/%s/deployments_config/environments/%s/variables", c.workspace, repoSlug, envUUID)
	for path != "" {
		var page struct {
			Values []DeploymentVariable `json:"values"`
			Next   string               `json:"next,omitempty"`
		}
		var fetchPath string
		if strings.HasPrefix(path, "http") {
			var err error
			fetchPath, err = relativePath(path, c.baseURL)
			if err != nil {
				return nil, err
			}
		} else {
			fetchPath = path
		}
		if err := c.do(ctx, http.MethodGet, fetchPath, nil, &page); err != nil {
			return nil, err
		}
		for i := range page.Values {
			if page.Values[i].UUID == varUUID {
				return &page.Values[i], nil
			}
		}
		path = page.Next
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Body: "deployment variable not found"}
}

func relativePath(fullURL, baseURL string) (string, error) {
	if !strings.HasPrefix(fullURL, baseURL) {
		return "", fmt.Errorf("unexpected pagination URL %q outside of base %q", fullURL, baseURL)
	}
	return strings.TrimPrefix(fullURL, baseURL), nil
}

// --- Default reviewers (repository) ----------------------------------------

func (c *Client) GetDefaultReviewer(ctx context.Context, repoSlug, selectedUser string) (*DefaultReviewer, error) {
	var out DefaultReviewer
	path := fmt.Sprintf("/repositories/%s/%s/default-reviewers/%s", c.workspace, repoSlug, selectedUser)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) AddDefaultReviewer(ctx context.Context, repoSlug, selectedUser string) (*DefaultReviewer, error) {
	var out DefaultReviewer
	path := fmt.Sprintf("/repositories/%s/%s/default-reviewers/%s", c.workspace, repoSlug, selectedUser)
	if err := c.do(ctx, http.MethodPut, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RemoveDefaultReviewer(ctx context.Context, repoSlug, selectedUser string) error {
	path := fmt.Sprintf("/repositories/%s/%s/default-reviewers/%s", c.workspace, repoSlug, selectedUser)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Default reviewers (project) --------------------------------------------

func (c *Client) GetProjectDefaultReviewer(ctx context.Context, projectKey, selectedUser string) (*DefaultReviewer, error) {
	var out DefaultReviewer
	path := fmt.Sprintf("/workspaces/%s/projects/%s/default-reviewers/%s", c.workspace, projectKey, selectedUser)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) AddProjectDefaultReviewer(ctx context.Context, projectKey, selectedUser string) (*DefaultReviewer, error) {
	var out DefaultReviewer
	path := fmt.Sprintf("/workspaces/%s/projects/%s/default-reviewers/%s", c.workspace, projectKey, selectedUser)
	if err := c.do(ctx, http.MethodPut, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RemoveProjectDefaultReviewer(ctx context.Context, projectKey, selectedUser string) error {
	path := fmt.Sprintf("/workspaces/%s/projects/%s/default-reviewers/%s", c.workspace, projectKey, selectedUser)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Project permissions (groups/users) -------------------------------------

func (c *Client) GetProjectGroupPermission(ctx context.Context, projectKey, groupSlug string) (*ProjectGroupPermission, error) {
	var out ProjectGroupPermission
	path := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/groups/%s", c.workspace, projectKey, groupSlug)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetProjectGroupPermission(ctx context.Context, projectKey, groupSlug, permission string) (*ProjectGroupPermission, error) {
	var out ProjectGroupPermission
	path := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/groups/%s", c.workspace, projectKey, groupSlug)
	body := &struct {
		Permission string `json:"permission"`
	}{Permission: permission}
	if err := c.do(ctx, http.MethodPut, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteProjectGroupPermission(ctx context.Context, projectKey, groupSlug string) error {
	path := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/groups/%s", c.workspace, projectKey, groupSlug)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) GetProjectUserPermission(ctx context.Context, projectKey, selectedUser string) (*ProjectUserPermission, error) {
	var out ProjectUserPermission
	path := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/users/%s", c.workspace, projectKey, selectedUser)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetProjectUserPermission(ctx context.Context, projectKey, selectedUser, permission string) (*ProjectUserPermission, error) {
	var out ProjectUserPermission
	path := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/users/%s", c.workspace, projectKey, selectedUser)
	body := &struct {
		Permission string `json:"permission"`
	}{Permission: permission}
	if err := c.do(ctx, http.MethodPut, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteProjectUserPermission(ctx context.Context, projectKey, selectedUser string) error {
	path := fmt.Sprintf("/workspaces/%s/projects/%s/permissions-config/users/%s", c.workspace, projectKey, selectedUser)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Webhooks (repository) --------------------------------------------------

func (c *Client) CreateHook(ctx context.Context, repoSlug string, h *Hook) (*Hook, error) {
	var out Hook
	path := fmt.Sprintf("/repositories/%s/%s/hooks", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPost, path, h, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetHook(ctx context.Context, repoSlug, uuid string) (*Hook, error) {
	var out Hook
	path := fmt.Sprintf("/repositories/%s/%s/hooks/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateHook(ctx context.Context, repoSlug, uuid string, h *Hook) (*Hook, error) {
	var out Hook
	path := fmt.Sprintf("/repositories/%s/%s/hooks/%s", c.workspace, repoSlug, uuid)
	if err := c.do(ctx, http.MethodPut, path, h, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteHook(ctx context.Context, repoSlug, uuid string) error {
	path := fmt.Sprintf("/repositories/%s/%s/hooks/%s", c.workspace, repoSlug, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Webhooks (workspace) ---------------------------------------------------

func (c *Client) CreateWorkspaceHook(ctx context.Context, h *Hook) (*Hook, error) {
	var out Hook
	path := fmt.Sprintf("/workspaces/%s/hooks", c.workspace)
	if err := c.do(ctx, http.MethodPost, path, h, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetWorkspaceHook(ctx context.Context, uuid string) (*Hook, error) {
	var out Hook
	path := fmt.Sprintf("/workspaces/%s/hooks/%s", c.workspace, uuid)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateWorkspaceHook(ctx context.Context, uuid string, h *Hook) (*Hook, error) {
	var out Hook
	path := fmt.Sprintf("/workspaces/%s/hooks/%s", c.workspace, uuid)
	if err := c.do(ctx, http.MethodPut, path, h, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteWorkspaceHook(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("/workspaces/%s/hooks/%s", c.workspace, uuid)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// --- Branching model (repository) -------------------------------------------

func (c *Client) GetBranchingModelSettings(ctx context.Context, repoSlug string) (*BranchingModelSettings, error) {
	var out BranchingModelSettings
	path := fmt.Sprintf("/repositories/%s/%s/branching-model/settings", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateBranchingModelSettings(ctx context.Context, repoSlug string, s *BranchingModelSettings) (*BranchingModelSettings, error) {
	var out BranchingModelSettings
	path := fmt.Sprintf("/repositories/%s/%s/branching-model/settings", c.workspace, repoSlug)
	if err := c.do(ctx, http.MethodPut, path, s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Branching model (project) ----------------------------------------------

func (c *Client) GetProjectBranchingModelSettings(ctx context.Context, projectKey string) (*BranchingModelSettings, error) {
	var out BranchingModelSettings
	path := fmt.Sprintf("/workspaces/%s/projects/%s/branching-model/settings", c.workspace, projectKey)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateProjectBranchingModelSettings(ctx context.Context, projectKey string, s *BranchingModelSettings) (*BranchingModelSettings, error) {
	var out BranchingModelSettings
	path := fmt.Sprintf("/workspaces/%s/projects/%s/branching-model/settings", c.workspace, projectKey)
	if err := c.do(ctx, http.MethodPut, path, s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Groups (legacy v1.0 API) ------------------------------------------------

func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	var out []Group
	path := fmt.Sprintf("/groups/%s/", c.workspace)
	if err := c.doV1(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetGroup finds one group by slug. The v1.0 API has no get-by-slug
// endpoint, only list, so this filters client-side.
func (c *Client) GetGroup(ctx context.Context, slug string) (*Group, error) {
	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].Slug == slug {
			return &groups[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Body: fmt.Sprintf("group %q not found", slug)}
}

func (c *Client) CreateGroup(ctx context.Context, g *Group) (*Group, error) {
	var out Group
	path := fmt.Sprintf("/groups/%s/", c.workspace)
	body := &struct {
		Name string `json:"name"`
	}{Name: g.Name}
	if err := c.doV1(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateGroup(ctx context.Context, slug string, g *Group) (*Group, error) {
	var out Group
	path := fmt.Sprintf("/groups/%s/%s/", c.workspace, slug)
	if err := c.doV1(ctx, http.MethodPut, path, g, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteGroup(ctx context.Context, slug string) error {
	path := fmt.Sprintf("/groups/%s/%s/", c.workspace, slug)
	return c.doV1(ctx, http.MethodDelete, path, nil, nil)
}

// --- Group membership (legacy v1.0 API) -------------------------------------

func (c *Client) ListGroupMembers(ctx context.Context, groupSlug string) ([]GroupMember, error) {
	var out []GroupMember
	path := fmt.Sprintf("/groups/%s/%s/members/", c.workspace, groupSlug)
	if err := c.doV1(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AddGroupMember(ctx context.Context, groupSlug, selectedUser string) error {
	path := fmt.Sprintf("/groups/%s/%s/members/%s/", c.workspace, groupSlug, selectedUser)
	return c.doV1(ctx, http.MethodPut, path, nil, nil)
}

func (c *Client) RemoveGroupMember(ctx context.Context, groupSlug, selectedUser string) error {
	path := fmt.Sprintf("/groups/%s/%s/members/%s/", c.workspace, groupSlug, selectedUser)
	return c.doV1(ctx, http.MethodDelete, path, nil, nil)
}

// IsGroupMember reports whether selectedUser is currently a member of
// groupSlug, since the v1.0 API has no single-member get endpoint.
func (c *Client) IsGroupMember(ctx context.Context, groupSlug, selectedUser string) (bool, error) {
	members, err := c.ListGroupMembers(ctx, groupSlug)
	if err != nil {
		return false, err
	}
	for _, m := range members {
		if m.UUID == selectedUser || m.Username == selectedUser {
			return true, nil
		}
	}
	return false, nil
}

// --- Users / workspace lookups ----------------------------------------------

func (c *Client) GetUser(ctx context.Context, selectedUser string) (*User, error) {
	var out User
	path := fmt.Sprintf("/users/%s", selectedUser)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var out User
	if err := c.do(ctx, http.MethodGet, "/user", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetWorkspace(ctx context.Context) (*Workspace, error) {
	var out Workspace
	path := fmt.Sprintf("/workspaces/%s", c.workspace)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListWorkspaceMembers returns every member of the client's workspace,
// following pagination.
func (c *Client) ListWorkspaceMembers(ctx context.Context) ([]User, error) {
	var users []User
	path := fmt.Sprintf("/workspaces/%s/members", c.workspace)
	for path != "" {
		var page struct {
			Values []workspaceMembership `json:"values"`
			Next   string                `json:"next,omitempty"`
		}
		fetchPath := path
		if strings.HasPrefix(path, "http") {
			var err error
			fetchPath, err = relativePath(path, c.baseURL)
			if err != nil {
				return nil, err
			}
		}
		if err := c.do(ctx, http.MethodGet, fetchPath, nil, &page); err != nil {
			return nil, err
		}
		for _, m := range page.Values {
			users = append(users, m.User)
		}
		path = page.Next
	}
	return users, nil
}
