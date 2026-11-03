# Changelog

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
