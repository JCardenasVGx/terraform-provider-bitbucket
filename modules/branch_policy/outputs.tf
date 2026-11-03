output "restriction_ids" {
  description = "IDs of every branch restriction this policy created, keyed by \"rule/pattern\"."
  value = merge(
    { for k, v in bitbucket_branch_restriction.require_approvals : "require_approvals/${k}" => v.id },
    { for k, v in bitbucket_branch_restriction.block_force_push : "block_force_push/${k}" => v.id },
    { for k, v in bitbucket_branch_restriction.block_delete : "block_delete/${k}" => v.id },
    { for k, v in bitbucket_branch_restriction.require_passing_builds : "require_passing_builds/${k}" => v.id },
    { for k, v in bitbucket_branch_restriction.restrict_push : "restrict_push/${k}" => v.id },
  )
}
