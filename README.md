# Terraform Provider for Bitbucket Cloud

A Terraform provider for managing Bitbucket Cloud as code: projects,
repositories, branch restrictions, Pipelines configuration (enable/disable,
variables, self-hosted runners) and deployment environments.

Full documentation (every resource/data source, attributes, examples,
import syntax) lives in [`docs/`](./docs) and is also published on the
[Terraform Registry](https://registry.terraform.io/providers/JCardenasVGx/bitbucket/latest/docs)
once released.

## Resources and data sources

| Name | Type | Manages |
|---|---|---|
| `bitbucket_project` | resource + data source | A Bitbucket project |
| `bitbucket_repository` | resource + data source | A Git repository |
| `bitbucket_branch_restriction` | resource | Branch protection rules (repository-level) |
| `bitbucket_pipeline_config` | resource | Enable/disable Pipelines on a repository |
| `bitbucket_pipeline_variable` | resource | Repository-level Pipelines variable |
| `bitbucket_workspace_pipeline_variable` | resource | Workspace-level Pipelines variable |
| `bitbucket_runner` | resource | Self-hosted runner registration (workspace-level) |
| `bitbucket_repository_runner` | resource | Self-hosted runner registration (repository-level) |
| `bitbucket_deployment_environment` | resource | Deployment environment (Test/Staging/Production) |
| `bitbucket_deployment_variable` | resource | Variable scoped to one deployment environment |
| `bitbucket_repository_group_permission` | resource | Workspace group permission grant on a repository |
| `bitbucket_repository_user_permission` | resource | User permission grant on a repository |
| `bitbucket_project_group_permission` | resource | Workspace group permission grant on a project |
| `bitbucket_project_user_permission` | resource | User permission grant on a project |
| `bitbucket_default_reviewer` | resource | Default PR reviewer on a repository |
| `bitbucket_project_default_reviewer` | resource | Default PR reviewer on a project |
| `bitbucket_group` | resource + data source | Workspace group (legacy v1.0 API) |
| `bitbucket_group_membership` | resource | Membership of a user in a workspace group |
| `bitbucket_hook` | resource | Webhook on a repository |
| `bitbucket_workspace_hook` | resource | Webhook on the workspace |
| `bitbucket_branching_model` | resource | Development/production branch config on a repository |
| `bitbucket_project_branching_model` | resource | Development/production branch config on a project |
| `bitbucket_workspace` | data source | The configured workspace |
| `bitbucket_user` / `bitbucket_current_user` | data source | Resolve a user's UUID by selector / the authenticated user |
| `bitbucket_groups` / `bitbucket_group_members` | data source | List groups / list a group's members |
| `bitbucket_workspace_members` | data source | List every member of the workspace |

Also included: [`modules/branch_policy`](./modules/branch_policy), a
reusable module that applies a standard set of branch restrictions to many
repositories from one place (the closest equivalent to a GitHub
organization ruleset, since Bitbucket Cloud has no public API for
project-level branch restrictions).

### Out of scope / known limitations

- **Project-level branch restrictions**: Bitbucket Cloud has this in its
  web UI, but there is no public API for it (only an internal, session-only
  endpoint used by the web app itself). It cannot be managed via IaC. Use
  `modules/branch_policy` to apply the same policy to every repository in
  a project instead.
- No deploy keys, SSH keys or GPG keys.
- No pipeline schedules, pipeline SSH keys/known hosts.
- No forked-repository resource, and deliberately no file-content
  (`commit_file`-style) resource — committing file content through
  Terraform state is an anti-pattern most teams should avoid.
- `bitbucket_branching_model` / `bitbucket_project_branching_model` only
  cover development/production branch settings, not custom branch-type
  prefixes (feature/bugfix/release/hotfix).

## Requirements

- Go 1.27+ (pinned in `go.mod`)
- Terraform 1.x
- A Bitbucket Cloud credential with Projects, Repositories, Pipelines and
  Runners scopes, depending on which resources you use — either:
  - an app password / Atlassian API token (`username` + `password`), or
  - a Repository/Project/Workspace Access Token (`token`) — except for
    `bitbucket_group`/`bitbucket_group_membership`, which need
    `username`+`password` regardless (see below)

## Using the provider

```hcl
terraform {
  required_providers {
    bitbucket = {
      source  = "registry.terraform.io/JCardenasVGx/bitbucket"
      version = "~> 1.0"
    }
  }
}

provider "bitbucket" {
  workspace = "my-workspace"
  # username and password can also be set via BITBUCKET_USERNAME /
  # BITBUCKET_PASSWORD instead of hardcoding them here.
}
```

Alternatively, authenticate with an Access Token instead of
username/password:

```hcl
provider "bitbucket" {
  workspace = "my-workspace"
  token     = var.bitbucket_access_token # or BITBUCKET_TOKEN
}
```

`token` is sent as `Authorization: Bearer <token>` and takes precedence
over `username`/`password` for everything **except**
`bitbucket_group`/`bitbucket_group_membership` (and the
`bitbucket_group`/`bitbucket_groups`/`bitbucket_group_members` data
sources): Bitbucket's legacy v1.0 groups API doesn't accept Access Tokens
at all, so those still require `username`/`password` even when `token` is
configured.

See [`examples/`](./examples) for a runnable example per resource/data
source, plus [`examples/complete`](./examples/complete) for everything
wired together and [`examples/org_policy`](./examples/org_policy) for the
`branch_policy` module applied across several repositories.

## Repository layout

```
main.go                      # provider entrypoint (providerserver.Serve)
internal/bitbucket/          # Bitbucket Cloud REST API client
internal/provider/           # resource/data source implementations
examples/                    # example .tf files (used by tfplugindocs and as reference)
modules/branch_policy/       # reusable module: apply branch restrictions to N repos at once
docs/                        # generated documentation (do not edit by hand, see below)
```

## Building and testing locally

```bash
go build -o bin/terraform-provider-bitbucket .
```

To point `terraform plan`/`apply` at this local build instead of a
registry, add to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "JCardenasVGx/bitbucket" = "/path/to/this/repo/bin"
  }
  direct {}
}
```

### Tests

```bash
go test ./...                 # unit tests (client, mocked HTTP)
TF_ACC=1 go test ./...        # also runs acceptance tests, against a local
                               # mock Bitbucket server -- no real credentials
                               # or network access needed
```

## Generating documentation

`docs/` is generated with [`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs)
from each schema's `Description` and the examples in `examples/`. Don't
edit `docs/*.md` by hand; regenerate instead:

```bash
go generate ./...
```

If you add a new resource/data source, add its example under
`examples/resources/<name>/resource.tf` (or
`examples/data-sources/<name>/data-source.tf`) first.

## Releasing

Tag a semver release and push it:

```bash
git tag v1.0.0
git push origin v1.0.0
```

A GitHub Actions workflow ([`.github/workflows/release.yml`](./.github/workflows/release.yml))
runs [GoReleaser](https://goreleaser.com) on tag push, which builds every
platform, signs the checksums with the GPG key configured in the repo's
`GPG_PRIVATE_KEY`/`PASSPHRASE` secrets, and publishes a GitHub Release with
all the required artifacts. The public Terraform Registry picks up new
releases automatically from there — publishing a provider update is just
pushing a tag.

## License

[Mozilla Public License v2.0](./LICENSE)
