package bot

import "strings"

func configuredBotUser(login string) bool {
	login = strings.TrimSpace(login)
	if login == "" || C.Github.BotUsers == "" {
		return false
	}
	for _, configured := range strings.Split(C.Github.BotUsers, ",") {
		if strings.EqualFold(strings.TrimSpace(configured), login) {
			return true
		}
	}
	return false
}

func configuredBotEventActor(m map[string]any, logins ...string) (string, bool) {
	candidates := append([]string{}, logins...)
	candidates = append(candidates,
		ext(m, "sender", "login"),
		ext(m, "workflow_run", "actor", "login"),
		ext(m, "workflow_run", "triggering_actor", "login"),
	)

	for _, login := range candidates {
		trimmed := strings.TrimSpace(login)
		if configuredBotUser(trimmed) {
			return trimmed, true
		}
	}
	return "", false
}

func botUserInteractionEvent(eventType string) bool {
	switch eventType {
	case "pull_request",
		"pull_request_review",
		"pull_request_review_comment",
		"issue_comment",
		"issues":
		return true
	default:
		return false
	}
}
