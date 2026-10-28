# Changelog

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
