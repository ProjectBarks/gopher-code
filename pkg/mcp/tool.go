package mcp

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Source: tools/MCPTool/MCPTool.ts, tools/MCPTool/classifyForCollapse.ts

// MaxResultSizeChars is the maximum result size for MCP tool output.
const MaxResultSizeChars = 100_000

// MCPTool wraps an MCP server tool so it can be used as a tools.Tool.
type MCPTool struct {
	client     *MCPClient
	info       ToolInfo
	serverName string
}

// NewMCPTool creates an MCPTool wrapping a server tool.
func NewMCPTool(client *MCPClient, serverName string, info ToolInfo) *MCPTool {
	return &MCPTool{client: client, serverName: serverName, info: info}
}

// Name returns the fully-qualified tool name: mcp__<server>__<tool>.
// Source: services/mcp/mcpStringUtils.ts:50-52
func (t *MCPTool) Name() string {
	return MCPToolPrefix(t.serverName) + NormalizeNameForMCP(t.info.Name)
}

// ServerName returns the originating MCP server name.
func (t *MCPTool) ServerName() string { return t.serverName }

// ToolName returns the raw tool name on the MCP server.
func (t *MCPTool) ToolName() string { return t.info.Name }

// Description returns the tool's description from the MCP server.
func (t *MCPTool) Description() string { return t.info.Description }

// InputSchema returns the JSON Schema for the tool's input.
func (t *MCPTool) InputSchema() json.RawMessage { return t.info.InputSchema }

// IsReadOnly returns true as a conservative default for MCP tools.
func (t *MCPTool) IsReadOnly() bool { return true }

// UserFacingName returns a display name: "toolName (serverName MCP)".
func (t *MCPTool) UserFacingName() string {
	return t.info.Name + " (" + t.serverName + " MCP)"
}

// Execute calls the tool on the MCP server and returns the result.
func (t *MCPTool) Execute(ctx context.Context, _ *tools.ToolContext, input json.RawMessage) (*tools.ToolOutput, error) {
	result, err := t.client.CallTool(ctx, t.info.Name, input)
	if err != nil {
		return tools.ErrorOutput("MCP call failed: " + err.Error()), nil
	}

	// Parse MCP tool call result content
	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		// If we can't parse structured content, return raw result
		raw := string(result)
		if len(raw) > MaxResultSizeChars {
			raw = raw[:MaxResultSizeChars] + "\n... [truncated]"
		}
		return tools.SuccessOutput(raw), nil
	}

	var text strings.Builder
	for _, c := range callResult.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}

	output := text.String()
	if len(output) > MaxResultSizeChars {
		output = output[:MaxResultSizeChars] + "\n... [truncated]"
	}

	if callResult.IsError {
		return tools.ErrorOutput(output), nil
	}
	return tools.SuccessOutput(output), nil
}

// ---------------------------------------------------------------------------
// Classify MCP tools for UI collapse grouping
// Source: tools/MCPTool/classifyForCollapse.ts
// ---------------------------------------------------------------------------

// MCPToolClassification indicates whether a tool is search-like, read-like,
// or neither (for UI collapse grouping).
type MCPToolClassification struct {
	IsSearch bool
	IsRead   bool
}

// camelToSnakeRe matches camelCase boundaries.
var camelToSnakeRe = regexp.MustCompile(`([a-z])([A-Z])`)

// normalizeMCPToolName converts camelCase/kebab-case to snake_case lowercase.
func normalizeMCPToolName(name string) string {
	s := camelToSnakeRe.ReplaceAllString(name, "${1}_${2}")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToLower(s)
}

// ClassifyMCPToolForCollapse classifies an MCP tool as search or read for
// UI collapse grouping. Unknown tools are conservative (both false).
// Source: classifyForCollapse.ts:595-604
func ClassifyMCPToolForCollapse(toolName string) MCPToolClassification {
	normalized := normalizeMCPToolName(toolName)
	return MCPToolClassification{
		IsSearch: mcpSearchTools[normalized],
		IsRead:   mcpReadTools[normalized],
	}
}

// mcpSearchTools is the set of MCP tool names classified as search operations.
// Source: classifyForCollapse.ts SEARCH_TOOLS
var mcpSearchTools = buildSet(
	// Slack
	"slack_search_public", "slack_search_public_and_private", "slack_search_channels", "slack_search_users",
	// GitHub
	"search_code", "search_repositories", "search_issues", "search_pull_requests", "search_orgs", "search_users",
	// Linear
	"search_documentation",
	// Datadog
	"search_logs", "search_spans", "search_rum_events", "search_audit_logs", "search_monitors", "search_monitor_groups", "find_slow_spans", "find_monitors_matching_pattern",
	// Sentry
	"search_docs", "search_events", "search_issue_events", "find_organizations", "find_teams", "find_projects", "find_releases", "find_dsns",
	// Notion
	"search",
	// Gmail
	"gmail_search_messages",
	// Google Drive
	"google_drive_search",
	// Google Calendar
	"gcal_find_my_free_time", "gcal_find_meeting_times", "gcal_find_user_emails",
	// Atlassian/Jira
	"search_jira_issues_using_jql", "search_confluence_using_cql", "lookup_jira_account_id",
	"confluence_search", "jira_search", "jira_search_fields",
	// Asana
	"asana_search_tasks", "asana_typeahead_search",
	// Filesystem
	"search_files",
	// Memory
	"search_nodes",
	// Brave
	"brave_web_search", "brave_local_search",
	// Grafana
	"search_dashboards", "search_folders",
	// Stripe
	"search_stripe_resources", "search_stripe_documentation",
	// PubMed
	"search_articles", "find_related_articles", "lookup_article_by_citation", "search_papers", "search_pubmed",
	"search_pubmed_key_words", "search_pubmed_advanced", "pubmed_search", "pubmed_mesh_lookup",
	// Firecrawl
	"firecrawl_search",
	// Exa
	"web_search_exa", "web_search_advanced_exa", "people_search_exa", "linkedin_search_exa", "deep_search_exa",
	// Perplexity
	"perplexity_search", "perplexity_search_web",
	// Tavily
	"tavily_search",
	// Obsidian
	"obsidian_simple_search", "obsidian_complex_search",
	// MongoDB
	"find", "search_knowledge",
	// Neo4j
	"search_memories", "find_memories_by_name",
	// Airtable
	"search_records",
	// Todoist
	"find_tasks", "find_tasks_by_date", "find_completed_tasks", "find_projects", "find_sections",
	"find_comments", "find_project_collaborators", "find_activity", "find_labels", "find_filters",
	// AWS
	"search_documentation", "search_catalog",
	// Terraform
	"search_modules", "search_providers", "search_policies",
)

// mcpReadTools is the set of MCP tool names classified as read operations.
// Source: classifyForCollapse.ts READ_TOOLS (abbreviated — full list in TS source)
var mcpReadTools = buildSet(
	// Slack
	"slack_read_channel", "slack_read_thread", "slack_read_canvas", "slack_read_user_profile",
	"slack_list_channels", "slack_get_channel_history", "slack_get_thread_replies",
	"slack_get_users", "slack_get_user_profile",
	// GitHub
	"get_me", "get_team_members", "get_teams", "get_commit", "get_file_contents",
	"get_repository_tree", "list_branches", "list_commits", "list_releases", "list_tags",
	"get_latest_release", "get_release_by_tag", "get_tag", "list_issues", "issue_read",
	"list_issue_types", "get_label", "list_label", "pull_request_read", "get_gist",
	"list_gists", "list_notifications", "get_notification_details", "projects_list",
	"projects_get", "actions_get", "actions_list", "get_job_logs",
	"get_code_scanning_alert", "list_code_scanning_alerts", "get_dependabot_alert",
	"list_dependabot_alerts", "get_secret_scanning_alert", "list_secret_scanning_alerts",
	"get_global_security_advisory", "list_global_security_advisories",
	"list_org_repository_security_advisories", "list_repository_security_advisories",
	"get_discussion", "get_discussion_comments", "list_discussion_categories", "list_discussions",
	"list_starred_repositories", "get_issue", "get_pull_request", "list_pull_requests",
	"get_pull_request_files", "get_pull_request_status", "get_pull_request_comments",
	"get_pull_request_reviews",
	// Linear
	"list_comments", "list_cycles", "get_document", "list_documents", "list_issue_statuses",
	"get_issue_status", "list_my_issues", "list_issue_labels", "list_projects", "get_project",
	"list_project_labels", "list_teams", "get_team", "list_users", "get_user",
	// Datadog
	"aggregate_logs", "list_spans", "aggregate_spans", "analyze_trace", "trace_critical_path",
	"query_metrics", "aggregate_rum_events", "list_rum_metrics", "get_rum_metric",
	"list_monitors", "get_monitor", "check_can_delete_monitor", "validate_monitor",
	"validate_existing_monitor", "list_dashboards", "get_dashboard", "query_dashboard_widget",
	"list_notebooks", "get_notebook", "query_notebook_cell", "get_profiling_metrics",
	"compare_profiling_metrics",
	// Sentry
	"whoami", "get_issue_details", "get_issue_tag_values", "get_trace_details",
	"get_event_attachment", "get_doc", "get_sentry_resource", "list_events", "list_issue_events",
	"get_sentry_issue",
	// Notion
	"fetch", "get_comments", "get_users", "get_self",
	// Gmail
	"gmail_get_profile", "gmail_read_message", "gmail_read_thread", "gmail_list_drafts",
	"gmail_list_labels",
	// Google Drive
	"google_drive_fetch", "google_drive_export",
	// Google Calendar
	"gcal_list_calendars", "gcal_list_events", "gcal_get_event",
	// Atlassian/Jira
	"atlassian_user_info", "get_accessible_atlassian_resources", "get_visible_jira_projects",
	"get_jira_project_issue_types_metadata", "get_jira_issue", "get_transitions_for_jira_issue",
	"get_jira_issue_remote_issue_links", "get_confluence_spaces", "get_confluence_page",
	"get_pages_in_confluence_space", "get_confluence_page_ancestors",
	"get_confluence_page_descendants", "get_confluence_page_footer_comments",
	"get_confluence_page_inline_comments",
	"confluence_get_page", "confluence_get_page_children", "confluence_get_comments",
	"confluence_get_labels", "jira_get_issue", "jira_get_transitions", "jira_get_worklog",
	"jira_get_agile_boards", "jira_get_board_issues", "jira_get_sprints_from_board",
	"jira_get_sprint_issues", "jira_get_link_types", "jira_download_attachments",
	"jira_batch_get_changelogs", "jira_get_user_profile", "jira_get_project_issues",
	"jira_get_project_versions",
	// Filesystem
	"read_file", "read_text_file", "read_media_file", "read_multiple_files",
	"list_directory", "list_directory_with_sizes", "directory_tree", "get_file_info",
	"list_allowed_directories",
	// Memory
	"read_graph", "open_nodes",
	// Postgres / SQLite
	"query", "read_query", "list_tables", "describe_table",
	// Git
	"git_status", "git_diff", "git_diff_unstaged", "git_diff_staged", "git_log",
	"git_show", "git_branch",
	// Grafana (abbreviated)
	"list_teams", "list_users_by_org", "get_dashboard_by_uid", "get_dashboard_summary",
	"get_dashboard_property", "get_dashboard_panel_queries", "run_panel_query",
	"list_datasources", "get_datasource", "get_query_examples", "query_prometheus",
	"query_prometheus_histogram", "list_prometheus_metric_metadata",
	"list_prometheus_metric_names", "list_prometheus_label_names",
	"list_prometheus_label_values", "query_loki_logs", "query_loki_stats",
	"query_loki_patterns", "list_loki_label_names", "list_loki_label_values",
	"list_incidents", "get_incident", "list_sift_investigations",
	"get_sift_investigation", "get_sift_analysis", "list_oncall_schedules",
	"get_oncall_shift", "get_current_oncall_users", "list_oncall_teams",
	"list_oncall_users", "list_alert_groups", "get_alert_group", "get_annotations",
	"get_annotation_tags", "get_panel_image",
	// Playwright
	"browser_console_messages", "browser_network_requests", "browser_take_screenshot",
	"browser_snapshot", "browser_get_config", "browser_route_list",
	"browser_cookie_list", "browser_cookie_get", "browser_localstorage_list",
	"browser_localstorage_get", "browser_sessionstorage_list",
	"browser_sessionstorage_get", "browser_storage_state",
	// Kubernetes
	"kubectl_get", "kubectl_describe", "kubectl_logs", "kubectl_context",
	"explain_resource", "list_api_resources", "namespaces_list", "nodes_log",
	"nodes_top", "pods_get", "pods_list", "pods_list_in_namespace",
	"pods_log", "pods_top", "resources_get", "resources_list",
)

func buildSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}
