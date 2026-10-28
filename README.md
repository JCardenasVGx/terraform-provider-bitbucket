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
- Webhooks, deploy keys and default reviewers are not implemented yet.

## Requirements

- Go 1.25+ (pinned in `go.mod`)
- Terraform 1.x
- A Bitbucket Cloud account with an API token or app password with
  Projects, Repositories, Pipelines and Runners scopes, depending on which
  resources you use

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
