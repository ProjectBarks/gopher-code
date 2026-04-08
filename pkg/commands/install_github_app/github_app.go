// Package install_github_app provides constants for the GitHub App installation workflow.
package install_github_app

// Source: constants/github-app.ts

// PRTitle is the default title for the GitHub Actions setup PR.
const PRTitle = "Add Claude Code GitHub Workflow"

// GitHubActionSetupDocsURL links to the setup documentation.
const GitHubActionSetupDocsURL = "https://github.com/anthropics/claude-code-action/blob/main/docs/setup.md"

// WorkflowContent is the default GitHub Actions workflow YAML for Claude Code.
// Source: constants/github-app.ts:6-56
const WorkflowContent = `name: Claude Code

on:
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]
  issues:
    types: [opened, assigned]
  pull_request_review:
    types: [submitted]

jobs:
  claude:
    if: |
      (github.event_name == 'issue_comment' && contains(github.event.comment.body, '@claude')) ||
      (github.event_name == 'pull_request_review_comment' && contains(github.event.comment.body, '@claude')) ||
      (github.event_name == 'pull_request_review' && contains(github.event.review.body, '@claude')) ||
      (github.event_name == 'issues' && (contains(github.event.issue.body, '@claude') || contains(github.event.issue.title, '@claude')))
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
      issues: read
      id-token: write
      actions: read
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Run Claude Code
        id: claude
        uses: anthropics/claude-code-action@v1
        with:
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
          additional_permissions: |
            actions: read
`
