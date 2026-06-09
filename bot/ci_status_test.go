package bot

import (
	"testing"

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
