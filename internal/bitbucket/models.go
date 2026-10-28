package bitbucket

// Project is the JSON representation of a Bitbucket Cloud project.
type Project struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPrivate   bool   `json:"is_private"`
	UUID        string `json:"uuid,omitempty"`
}

// Repository is the JSON representation of a Bitbucket Cloud repository.
type Repository struct {
	Slug        string          `json:"slug,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	IsPrivate   *bool           `json:"is_private,omitempty"`
	ForkPolicy  string          `json:"fork_policy,omitempty"`
	Language    string          `json:"language,omitempty"`
	SCM         string          `json:"scm,omitempty"`
	HasIssues   *bool           `json:"has_issues,omitempty"`
	HasWiki     *bool           `json:"has_wiki,omitempty"`
	Project     *ProjectLink    `json:"project,omitempty"`
	UUID        string          `json:"uuid,omitempty"`
	MainBranch  *MainBranchLink `json:"mainbranch,omitempty"`
}

// ProjectLink references a project by key when attaching it to a repository.
type ProjectLink struct {
	Key string `json:"key"`
}

// MainBranchLink lets callers set/read the repository's default branch.
type MainBranchLink struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// BranchRestriction is the JSON representation of a Bitbucket Cloud
// branch-restriction rule.
type BranchRestriction struct {
	ID              int64              `json:"id,omitempty"`
	Kind            string             `json:"kind"`
	BranchMatchKind string             `json:"branch_match_kind,omitempty"`
	Pattern         string             `json:"pattern,omitempty"`
	BranchType      string             `json:"branch_type,omitempty"`
	Value           *int64             `json:"value,omitempty"`
	Users           []AccountReference `json:"users,omitempty"`
	Groups          []GroupReference   `json:"groups,omitempty"`
}

// AccountReference identifies a Bitbucket account by UUID.
type AccountReference struct {
	UUID string `json:"uuid"`
}

// GroupReference identifies a workspace group by slug.
type GroupReference struct {
	Slug string `json:"slug"`
}

// PipelinesConfig toggles Pipelines on/off for a repository.
type PipelinesConfig struct {
	Enabled bool `json:"enabled"`
}

// PipelineVariable is a repository- or workspace-level pipeline variable.
type PipelineVariable struct {
	UUID    string `json:"uuid,omitempty"`
	Key     string `json:"key,omitempty"`
	Value   string `json:"value,omitempty"`
	Secured bool   `json:"secured"`
}

// DeploymentVariable is a variable scoped to one deployment environment.
type DeploymentVariable struct {
	UUID    string `json:"uuid,omitempty"`
	Key     string `json:"key,omitempty"`
	Value   string `json:"value,omitempty"`
	Secured bool   `json:"secured"`
}

// Runner is a self-hosted Pipelines runner registration (workspace- or
// repository-scoped). Bitbucket only returns OauthClient.Secret once, at
// creation time.
type Runner struct {
	UUID        string             `json:"uuid,omitempty"`
	Name        string             `json:"name,omitempty"`
	Labels      []string           `json:"labels,omitempty"`
	State       *RunnerState       `json:"state,omitempty"`
	OauthClient *RunnerOauthClient `json:"oauth_client,omitempty"`
}

type RunnerState struct {
	Status string `json:"status,omitempty"`
}

type RunnerOauthClient struct {
	ID     string `json:"id,omitempty"`
	Secret string `json:"secret,omitempty"`
}

type createRunnerRequest struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
}

// DeploymentEnvironment is a Pipelines deployment environment
// (Test/Staging/Production or Custom) on a repository.
type DeploymentEnvironment struct {
	UUID            string                   `json:"uuid,omitempty"`
	Name            string                   `json:"name,omitempty"`
	EnvironmentType *EnvironmentType         `json:"environment_type,omitempty"`
	Restrictions    *EnvironmentRestrictions `json:"restrictions,omitempty"`
}

type EnvironmentType struct {
	Name string `json:"name"`
}

type EnvironmentRestrictions struct {
	AdminOnly bool `json:"admin_only"`
}

type environmentChangeRequest struct {
	Change *environmentChange `json:"change"`
}

type environmentChange struct {
	Name         string                   `json:"name,omitempty"`
	Restrictions *EnvironmentRestrictions `json:"restrictions,omitempty"`
}
