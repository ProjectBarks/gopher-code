package compact

import (
	"regexp"
	"strings"
)

// Source: services/compact/prompt.ts

// CompactMaxOutputTokens is the max tokens for the compact summary response.
// Source: utils/context.ts (COMPACT_MAX_OUTPUT_TOKENS)
const CompactMaxOutputTokens = 8192

// NoToolsPreamble prevents the model from calling tools during compaction.
// Source: services/compact/prompt.ts:19-26
const noToolsPreamble = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.

- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- You already have all the context you need in the conversation above.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

`

// noToolsTrailer reinforces the no-tools instruction at the end.
// Source: services/compact/prompt.ts:269-272
const noToolsTrailer = "\n\nREMINDER: Do NOT call any tools. Respond with plain text only — " +
	"an <analysis> block followed by a <summary> block. " +
	"Tool calls will be rejected and you will fail the task."

// baseCompactPrompt is the full-conversation summarization prompt.
// Source: services/compact/prompt.ts:61-143
const baseCompactPrompt = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like:
     - file names
     - full code snippets
     - function signatures
     - file edits
   - Errors that you ran into and how you fixed them
   - Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to the most recent messages and include full code snippets where applicable and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that you ran into, and how you fixed them. Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback and changing intent.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying special attention to the most recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that you will take that is related to the most recent work you were doing. IMPORTANT: ensure that this step is DIRECTLY in line with the user's most recent explicit requests, and the task you were working on immediately before this summary request. If your last task was concluded, then only list next steps if they are explicitly in line with the users request. Do not start on tangential requests or really old requests that were already completed without confirming with the user first.
                       If there is a next step, include direct quotes from the most recent conversation showing exactly what task you were working on and where you left off. This should be verbatim to ensure there's no drift in task interpretation.

Please provide your summary based on the conversation so far, following this structure and ensuring precision and thoroughness in your response.

There may be additional summarization instructions provided in the included context. If so, remember to follow these instructions when creating the above summary. Examples of instructions include:
<example>
## Compact Instructions
When summarizing the conversation focus on typescript code changes and also remember the mistakes you made and how you fixed them.
</example>

<example>
# Summary instructions
When you are using compact - please focus on test output and code changes. Include file reads verbatim.
</example>
`

// GetCompactPrompt builds the compaction system prompt with optional custom instructions.
// Source: services/compact/prompt.ts:293-303
func GetCompactPrompt(customInstructions string) string {
	prompt := noToolsPreamble + baseCompactPrompt

	if strings.TrimSpace(customInstructions) != "" {
		prompt += "\n\nAdditional Instructions:\n" + customInstructions
	}

	prompt += noToolsTrailer
	return prompt
}

// analysisRE matches <analysis>...</analysis> blocks for stripping.
var analysisRE = regexp.MustCompile(`(?s)<analysis>.*?</analysis>`)

// summaryRE matches <summary>...</summary> blocks for extraction.
var summaryRE = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)

// multiNewlineRE collapses multiple blank lines.
var multiNewlineRE = regexp.MustCompile(`\n\n+`)

// FormatCompactSummary strips the <analysis> scratchpad and formats the <summary>.
// Source: services/compact/prompt.ts:311-335
func FormatCompactSummary(summary string) string {
	result := summary

	// Strip analysis section
	// Source: services/compact/prompt.ts:316-319
	result = analysisRE.ReplaceAllString(result, "")

	// Extract and format summary section
	// Source: services/compact/prompt.ts:322-329
	if m := summaryRE.FindStringSubmatch(result); m != nil {
		content := strings.TrimSpace(m[1])
		result = summaryRE.ReplaceAllString(result, "Summary:\n"+content)
	}

	// Clean up extra whitespace
	// Source: services/compact/prompt.ts:332
	result = multiNewlineRE.ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

// GetCompactUserSummaryMessage builds the post-compaction user message.
// Source: services/compact/prompt.ts:337-374
func GetCompactUserSummaryMessage(summary string, suppressFollowUp bool, transcriptPath string, recentPreserved bool) string {
	formatted := FormatCompactSummary(summary)

	base := "This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n" + formatted

	if transcriptPath != "" {
		base += "\n\nIf you need specific details from before compaction (like exact code snippets, error messages, or content you generated), read the full transcript at: " + transcriptPath
	}

	if recentPreserved {
		base += "\n\nRecent messages are preserved verbatim."
	}

	if suppressFollowUp {
		return base + "\nContinue the conversation from where it left off without asking the user any further questions. Resume directly — do not acknowledge the summary, do not recap what was happening, do not preface with \"I'll continue\" or similar. Pick up the last task as if the break never happened."
	}

	return base
}
