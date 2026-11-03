variable "repository_slug" {
  type        = string
  description = "Slug of the repository this policy applies to."
}

variable "branch_patterns" {
  type        = list(string)
  description = "Branch name or glob patterns this policy applies to, e.g. [\"main\", \"release/*\"]."
  default     = ["main"]
}

variable "require_approvals" {
  type        = number
  description = "Minimum number of approvals required to merge. Set to 0 to disable this rule."
  default     = 1
}

variable "block_force_push" {
  type        = bool
  description = "Block force-pushes to the matched branches."
  default     = true
}

variable "block_delete" {
  type        = bool
  description = "Block deleting the matched branches."
  default     = true
}

variable "require_passing_builds" {
  type        = bool
  description = "Require Pipelines builds to pass before merging. Only useful if bitbucket_pipeline_config.enabled = true on the repository."
  default     = false
}

variable "restrict_push_to_groups" {
  type        = list(string)
  description = "If non-empty, blocks direct pushes to the matched branches except for these workspace group slugs (e.g. [\"release-managers\"]). Leave empty to not restrict who can push."
  default     = []
}

variable "restrict_push_to_users" {
  type        = list(string)
  description = "If non-empty, blocks direct pushes to the matched branches except for these user UUIDs. Leave empty to not restrict who can push."
  default     = []
}
