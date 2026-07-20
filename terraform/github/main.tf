resource "github_repository_ruleset" "main_protection" {
  name        = "mirror"
  repository  = "hippocampus"
  target      = "branch"
  enforcement = "active"

  conditions {
    ref_name {
      include = ["refs/heads/mirror"]
      exclude = []
    }
  }

  rules {
    merge_queue {
      check_response_timeout_minutes    = 60
      grouping_strategy                 = "ALLGREEN"
      max_entries_to_build              = 5
      max_entries_to_merge              = 5
      merge_method                      = "MERGE"
      min_entries_to_merge              = 1
      min_entries_to_merge_wait_minutes = 5
    }
  }
}
