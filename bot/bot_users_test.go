package bot

import "testing"

func TestConfiguredBotUserTrimsCommaSeparatedList(t *testing.T) {
	oldBotUsers := C.Github.BotUsers
	t.Cleanup(func() {
		C.Github.BotUsers = oldBotUsers
	})

	C.Github.BotUsers = "dependabot[bot], github-advanced-security[bot] , Silent-Dev"

	if !configuredBotUser("github-advanced-security[bot]") {
		t.Fatal("expected bot user with surrounding config spaces to match")
	}
	if !configuredBotUser("silent-dev") {
		t.Fatal("expected bot user match to be case-insensitive")
	}
	if configuredBotUser("github-actions[bot]") {
		t.Fatal("did not expect unconfigured bot user to match")
	}
}

func TestConfiguredBotEventActorChecksWorkflowRunActors(t *testing.T) {
	oldBotUsers := C.Github.BotUsers
	t.Cleanup(func() {
		C.Github.BotUsers = oldBotUsers
	})

	C.Github.BotUsers = "github-advanced-security[bot]"
	payload := map[string]any{
		"sender": map[string]any{
			"login": "github-actions[bot]",
		},
		"workflow_run": map[string]any{
			"actor": map[string]any{
				"login": "github-advanced-security[bot]",
			},
			"triggering_actor": map[string]any{
				"login": "human-user",
			},
		},
	}

	got, ok := configuredBotEventActor(payload, "human-user")
	if !ok {
		t.Fatal("expected workflow_run actor to match configured bot user")
	}
	if got != "github-advanced-security[bot]" {
		t.Fatalf("matched actor = %q, want github-advanced-security[bot]", got)
	}
}

func TestConfiguredBotEventActorChecksExplicitLogins(t *testing.T) {
	oldBotUsers := C.Github.BotUsers
	t.Cleanup(func() {
		C.Github.BotUsers = oldBotUsers
	})

	C.Github.BotUsers = "github-advanced-security[bot]"
	payload := map[string]any{
		"sender": map[string]any{
			"login": "github-actions[bot]",
		},
	}

	got, ok := configuredBotEventActor(payload, "github-advanced-security[bot]")
	if !ok {
		t.Fatal("expected explicit login to match configured bot user")
	}
	if got != "github-advanced-security[bot]" {
		t.Fatalf("matched actor = %q, want github-advanced-security[bot]", got)
	}
}

func TestBotUserInteractionEventExcludesCIEvents(t *testing.T) {
	for _, eventType := range []string{"workflow_run", "workflow_job", "check_run", "check_suite"} {
		if botUserInteractionEvent(eventType) {
			t.Fatalf("did not expect %s to be allowed for configured bot users", eventType)
		}
	}

	for _, eventType := range []string{"pull_request", "pull_request_review", "pull_request_review_comment", "issue_comment", "issues"} {
		if !botUserInteractionEvent(eventType) {
			t.Fatalf("expected %s to be allowed for configured bot users", eventType)
		}
	}
}
