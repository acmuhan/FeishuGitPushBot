package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRenderCIStatusCleansWorkflowTitlePrefix(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "✅ Workflow Succeeded: CI",
			Status:       "completed",
			Conclusion:   "success",
			RunID:        12345,
			Duration:     "57 seconds",
		},
	}, "https://github.com/NCUHOME/youth-pen")

	assert.Contains(t, got, "✅ CI **passed**")
	assert.NotContains(t, got, "✅ ✅")
	assert.NotContains(t, got, "Workflow Succeeded")
}

func TestRenderCIStatusCleansCompactWorkflowTitlePrefix(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "✅Workflow Succeeded: Scheduled",
			Status:       "completed",
			Conclusion:   "success",
			RunID:        12345,
			Duration:     "1 minute 12 seconds",
		},
	}, "https://github.com/NCUHOME/putable")

	assert.Contains(t, got, "✅ Scheduled **passed**")
	assert.NotContains(t, got, "✅ ✅")
	assert.NotContains(t, got, "Workflow Succeeded")
}

func TestRenderCIStatusCleansNestedWorkflowTitlePrefixes(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "✅ Workflow Succeeded: ✅Workflow Succeeded: Scheduled",
			Status:       "completed",
			Conclusion:   "success",
			RunID:        12345,
		},
	}, "")

	assert.Equal(t, "✅ Scheduled **passed**", got)
}

func TestRenderCIStatusCleansUnknownWorkflowTitlePrefix(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "⚠️ Workflow Startup Failure: Scheduled",
			Status:       "completed",
			Conclusion:   "startup_failure",
			RunID:        12345,
		},
	}, "")

	assert.Equal(t, "⚠️ Scheduled **startup failure**", got)
}

func TestMakeCIActionButtonsCleansWorkflowTitlePrefix(t *testing.T) {
	got := makeCIActionButtons([]CIStatus{
		{
			WorkflowName: "❌ Workflow Failed: ✅Workflow Succeeded: Scheduled",
			Status:       "completed",
			Conclusion:   "failure",
			RunID:        12345,
		},
	}, "https://github.com/NCUHOME/putable")

	assert.Len(t, got, 2)
	assert.Equal(t, "View Scheduled Logs", got[0].Text)
}

func TestRenderCIStatusKeepsWorkflowNameThatStartsWithWorkflow(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "Workflow Builder: Test",
			Status:       "completed",
			Conclusion:   "success",
		},
	}, "")

	assert.Equal(t, "✅ Workflow Builder: Test **passed**", got)
}

func TestRenderCIStatusLifecycleStates(t *testing.T) {
	statuses := []CIStatus{
		{WorkflowName: "CI", Status: "requested"},
		{WorkflowName: "CI", Status: "queued"},
		{WorkflowName: "CI", Status: "in_progress"},
		{WorkflowName: "CI", Status: "completed", Conclusion: "success"},
	}
	got := make([]string, 0, len(statuses))
	for _, status := range statuses {
		got = append(got, renderCIStatuses([]CIStatus{status}, ""))
	}

	assert.Equal(t, "⚙️ CI **requested**", got[0])
	assert.Equal(t, "🕒 CI **queued**", got[1])
	assert.Equal(t, "⏳ CI **running**", got[2])
	assert.Equal(t, "✅ CI **passed**", got[3])
}

func TestRenderCIStatusUnknownConclusionIsNotSuccess(t *testing.T) {
	statuses := []CIStatus{
		{WorkflowName: "CI", Status: "completed", Conclusion: "startup_failure", RunID: 12345},
	}
	got := renderCIStatuses(statuses, "https://github.com/test/repo")

	assert.Equal(t, "⚠️ CI **startup failure** ([logs](https://github.com/test/repo/actions/runs/12345))", got)
	assert.NotContains(t, got, "✅")
	assert.True(t, ciFailed(statuses))
	assert.Len(t, makeCIActionButtons(statuses, "https://github.com/test/repo"), 2)
}

func TestRenderCIStatusGroupsJobsUnderWorkflow(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "job:222",
			JobName:      "Code Quality",
			Status:       "in_progress",
			RunID:        12345,
			ParentRunID:  12345,
		},
		{
			WorkflowName: "CI",
			Status:       "in_progress",
			RunID:        12345,
		},
		{
			WorkflowName: "job:111",
			JobName:      "Build",
			Status:       "completed",
			Conclusion:   "success",
			RunID:        12345,
			ParentRunID:  12345,
			Duration:     "57 seconds",
		},
	}, "https://github.com/NCUHOME/youth-pen")

	assert.Contains(t, got, "⏳ CI **running** ([logs](https://github.com/NCUHOME/youth-pen/actions/runs/12345))")
	assert.Contains(t, got, "↳ ⏳ Code Quality **running**")
	assert.Contains(t, got, "↳ ✅ Build **passed** (57 seconds)")
	assert.NotContains(t, got, "J621111")
}

func TestRenderCIStatusOrdersOrphanJobsDeterministically(t *testing.T) {
	got := renderCIStatuses([]CIStatus{
		{
			WorkflowName: "job:222",
			JobName:      "Later Run",
			Status:       "queued",
			ParentRunID:  200,
		},
		{
			WorkflowName: "job:111",
			JobName:      "Earlier Run",
			Status:       "queued",
			ParentRunID:  100,
		},
	}, "")

	assert.Equal(t, "🕒 Earlier Run **queued**<br>🕒 Later Run **queued**", got)
}

func TestProcessCommitMessageDoesNotBoldConventionalPrefix(t *testing.T) {
	got := ProcessCommitMessage("docs:add weekly manual report", "")

	assert.Equal(t, "docs: add weekly manual report", got)
	assert.NotContains(t, got, "**docs:**")
}

func TestBranchPushUsesDividerBetweenMergedPushes(t *testing.T) {
	detail := EventDetail{
		Title:         "🍏 Branch Push",
		RefName:       "feat/controlpanel",
		RepoName:      "NCUHOME/NCU_Medical_Agent",
		RepoURL:       "https://github.com/NCUHOME/NCU_Medical_Agent",
		Text:          "🔸 docs: add weekly manual report ([f6754ed](https://github.com/NCUHOME/NCU_Medical_Agent/commit/f6754ed))" + pushGroupSeparator + "🔸 chore: capture workflow artifacts and updates ([3437a98](https://github.com/NCUHOME/NCU_Medical_Agent/commit/3437a98))",
		Action:        "push",
		CommitCount:   2,
		EventTime:     time.Now().Format(time.RFC3339),
		EventTimeEnd:  time.Now().Format(time.RFC3339),
		AuthorLogins:  []string{"NEKO-CwC"},
		AuthorAvatars: []string{"https://avatars.githubusercontent.com/NEKO-CwC"},
	}

	card := BuildCard(context.Background(), detail.RepoName, "NEKO-CwC", "https://github.com/NEKO-CwC", "", detail)
	cardJSON := card.String()

	assert.Contains(t, cardJSON, `"tag":"hr"`)
	assert.NotContains(t, cardJSON, "\\n---\\n")
	assert.NotContains(t, cardJSON, "**docs:**")
}

func TestNormalizeLiteralNewlinesOutsideFences(t *testing.T) {
	input := "Line1\\nLine2\n```go\ncode\\nhere\n```\nLine3"
	got := normalizeLiteralNewlinesOutsideFences(input)

	if !strings.Contains(got, "Line1\nLine2\n") {
		t.Fatalf("literal \\n outside fence should be normalized: %q", got)
	}
	if !strings.Contains(got, "code\\nhere") {
		t.Fatalf("literal \\n inside fence should be preserved: %q", got)
	}
}

func TestNormalizeLiteralNewlinesPreservesFenceContent(t *testing.T) {
	input := "Before\n```\nline1\\nline2\n```\nAfter"
	got := normalizeLiteralNewlinesOutsideFences(input)

	if !strings.Contains(got, "Before\n") {
		t.Fatalf("text before fence should have real newline: %q", got)
	}
	if !strings.Contains(got, "line1\\nline2") {
		t.Fatalf("literal \\n inside fence should be preserved: %q", got)
	}
	if !strings.Contains(got, "\nAfter") {
		t.Fatalf("text after fence should have real newline: %q", got)
	}
}

func TestNormalizeMarkdownHeadingsDowngradesHeadings(t *testing.T) {
	input := "## Title\nSome text\n### Subtitle"
	got := normalizeMarkdownHeadings(input)

	if strings.Contains(got, "##") {
		t.Fatalf("heading markers should be removed: %q", got)
	}
	if !strings.Contains(got, "Title\nSome text\nSubtitle") {
		t.Fatalf("heading content should be preserved: %q", got)
	}
}

func TestNormalizeMarkdownHeadingsSkipsContentInsideFences(t *testing.T) {
	input := "Before\n```\n## Not a heading\n```\n## Real heading"
	got := normalizeMarkdownHeadings(input)

	if strings.Contains(got, "Not a heading") {
		fenceContent := "## Not a heading"
		if !strings.Contains(got, fenceContent) {
			t.Fatalf("fence content should be preserved as-is: %q", got)
		}
	}
	if strings.Contains(got, "## Real heading") {
		t.Fatalf("heading outside fence should be downgraded: %q", got)
	}
}

func TestIsFenceLine(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"```", true},
		{"```go", true},
		{"~~~", true},
		{"~~~python", true},
		{"  ```  ", true},
		{"not a fence", false},
		{"`` not enough", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isFenceLine(tt.input); got != tt.want {
			t.Errorf("isFenceLine(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
