# Changelog

## v1.3.0

### Added

- Provider: new `token` attribute (also `BITBUCKET_TOKEN`) to authenticate
  with a Repository/Project/Workspace Access Token via
  `Authorization: Bearer` instead of HTTP Basic Auth. Takes precedence over
  `username`/`password` for every resource except `bitbucket_group` and
  `bitbucket_group_membership`, which use Bitbucket's legacy v1.0 groups
  API and don't accept Access Tokens at all — those two (and the
  `bitbucket_group`/`bitbucket_groups`/`bitbucket_group_members` data
  sources) still need `username`/`password` even when `token` is set.

## v1.2.0

### Added

Resources:

- `bitbucket_project_group_permission` / `bitbucket_project_user_permission`:
  permission grants at the project level (applies to every repository
  inside it), mirroring the existing repository-level equivalents.
- `bitbucket_default_reviewer` / `bitbucket_project_default_reviewer`:
  default pull request reviewers, at the repository or project level.
- `bitbucket_group` / `bitbucket_group_membership`: manage workspace
  groups and their members (via Bitbucket Cloud's legacy v1.0 API, which
  has no v2.0 replacement for this).
- `bitbucket_hook` / `bitbucket_workspace_hook`: webhooks at the
  repository or workspace level.
- `bitbucket_branching_model` / `bitbucket_project_branching_model`:
  development/production branch configuration, at the repository or
  project level.

Data sources (mainly to resolve UUIDs/slugs instead of hardcoding them):

- `bitbucket_workspace`, `bitbucket_user`, `bitbucket_current_user`
- `bitbucket_group`, `bitbucket_groups`, `bitbucket_group_members`
- `bitbucket_workspace_members`

### Known limitations (unchanged from v1.1.0, still out of scope)

- No deploy keys, SSH keys or GPG keys.
- No pipeline schedules, pipeline SSH keys/known hosts.
- No forked-repository or file-content (`commit_file`-style) resources —
  the latter is a deliberate omission: committing file content through
  Terraform state is an anti-pattern most teams should avoid.
- `bitbucket_branching_model`/`bitbucket_project_branching_model` only
  cover development/production branch settings, not custom branch-type
  prefixes (feature/bugfix/release/hotfix).
- No project-level branch restrictions (Bitbucket Cloud has no public API
  for it) — use `modules/branch_policy` to apply the same policy to every
  repository in a project instead.

## v1.1.0

### Added

- `bitbucket_branch_restriction`: new `users` and `groups` attributes to
  scope a restriction to specific users/groups (e.g. who is allowed to
  push directly on a `push` kind restriction).
- `bitbucket_repository_group_permission`: grants a workspace group a
  permission level (read/write/admin) on a repository.
- `bitbucket_repository_user_permission`: grants a user a permission
  level (read/write/admin) on a repository.
- `modules/branch_policy`: new `restrict_push_to_groups` /
  `restrict_push_to_users` variables to block direct pushes except for
  specific groups/users.

## v1.0.0

Initial release.

### Resources

- `bitbucket_project`
- `bitbucket_repository`
- `bitbucket_branch_restriction`
- `bitbucket_pipeline_config`
- `bitbucket_pipeline_variable`
- `bitbucket_workspace_pipeline_variable`
- `bitbucket_runner`
- `bitbucket_repository_runner`
- `bitbucket_deployment_environment`
- `bitbucket_deployment_variable`

### Data sources

- `bitbucket_project`
- `bitbucket_repository`

### Other

- `modules/branch_policy`: applies a standard set of branch restrictions
  to many repositories from one place.
- Documentation generated with `tfplugindocs` in `docs/`.

### Known limitations

- No support for project-level branch restrictions (Bitbucket Cloud has no
  public API for it).
- No webhooks, deploy keys or default reviewers yet.
