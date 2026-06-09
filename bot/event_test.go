package bot

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
)

// helper functions to create pointers
func strPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64 { return &i }
func tsPtr(t time.Time) *github.Timestamp {
	ts := github.Timestamp{Time: t}
	return &ts
}

func TestParseEventNewTypes(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name      string
		event     interface{}
		eventType string
		wantTitle string
		wantSkip  bool
	}{
		// CommitCommentEvent
		{
			name: "commit_comment created",
			event: &github.CommitCommentEvent{
				Action: strPtr("created"),
				Comment: &github.RepositoryComment{
					ID:     int64Ptr(12345),
					Body:   strPtr("This is a commit comment"),
					HTMLURL: strPtr("https://github.com/test/repo/commit/abc123#commitcomment-12345"),
					CreatedAt: &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "commit_comment",
			wantTitle: "💬 Commit Comment created",
		},
		// DeploymentEvent
		{
			name: "deployment created",
			event: &github.DeploymentEvent{
				Deployment: &github.Deployment{
					ID:          int64Ptr(100),
					Ref:         strPtr("main"),
					Environment: strPtr("production"),
					Description: strPtr("Deploy to production"),
					Task:        strPtr("deploy"),
					URL:         strPtr("https://api.github.com/repos/test/repo/deployments/100"),
					CreatedAt:   &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "deployment",
			wantTitle: "🚀 Deployment Created",
		},
		// DeploymentStatusEvent
		{
			name: "deployment_status success",
			event: &github.DeploymentStatusEvent{
				Action: strPtr("created"),
				Deployment: &github.Deployment{
					ID:          int64Ptr(100),
					Environment: strPtr("production"),
				},
				DeploymentStatus: &github.DeploymentStatus{
					ID:          int64Ptr(200),
					State:       strPtr("success"),
					Description: strPtr("Deployment succeeded"),
					TargetURL:   strPtr("https://example.com"),
					CreatedAt:   &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "deployment_status",
			wantTitle: "✅ Deployment Success",
		},
		// DiscussionEvent
		{
			name: "discussion created",
			event: &github.DiscussionEvent{
				Action: strPtr("created"),
				Discussion: &github.Discussion{
					ID:        int64Ptr(500),
					Number:    intPtr(42),
					Title:     strPtr("How to deploy?"),
					Body:      strPtr("I need help with deployment"),
					HTMLURL:   strPtr("https://github.com/test/repo/discussions/42"),
					CreatedAt: &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "discussion",
			wantTitle: "💬 New Discussion",
		},
		// DiscussionCommentEvent
		{
			name: "discussion_comment created",
			event: &github.DiscussionCommentEvent{
				Action: strPtr("created"),
				Discussion: &github.Discussion{
					Number: intPtr(42),
					Title:  strPtr("How to deploy?"),
				},
				Comment: &github.CommentDiscussion{
					ID:        int64Ptr(600),
					Body:      strPtr("Try using Docker"),
					HTMLURL:   strPtr("https://github.com/test/repo/discussions/42#discussioncomment-600"),
					CreatedAt: &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "discussion_comment",
			wantTitle: "💬 Discussion Comment created",
		},
		// LabelEvent
		{
			name: "label created",
			event: &github.LabelEvent{
				Action: strPtr("created"),
				Label: &github.Label{
					ID:          int64Ptr(700),
					Name:        strPtr("bug"),
					Description: strPtr("Something isn't working"),
					Color:       strPtr("d73a4a"),
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "label",
			wantTitle: "🏷️ Label created: bug",
		},
		// MilestoneEvent
		{
			name: "milestone created",
			event: &github.MilestoneEvent{
				Action: strPtr("created"),
				Milestone: &github.Milestone{
					ID:           int64Ptr(800),
					Number:       intPtr(1),
					Title:        strPtr("v1.0"),
					Description:  strPtr("First release"),
					State:        strPtr("open"),
					OpenIssues:   intPtr(5),
					ClosedIssues: intPtr(2),
					HTMLURL:      strPtr("https://github.com/test/repo/milestone/1"),
					CreatedAt:    &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "milestone",
			wantTitle: "🎯 Milestone created: v1.0",
		},
		// PullRequestReviewThreadEvent
		{
			name: "pull_request_review_thread resolved",
			event: &github.PullRequestReviewThreadEvent{
				Action: strPtr("resolved"),
				Thread: &github.PullRequestThread{
					ID: int64Ptr(900),
				},
				PullRequest: &github.PullRequest{
					Number: intPtr(10),
					Title:  strPtr("Fix bug"),
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "pull_request_review_thread",
			wantTitle: "✅ PR Review Thread Resolved",
		},
		// StatusEvent
		{
			name: "status success",
			event: &github.StatusEvent{
				SHA:         strPtr("abc1234567890"),
				State:       strPtr("success"),
				Context:     strPtr("ci/build"),
				Description: strPtr("Build succeeded"),
				TargetURL:   strPtr("https://ci.example.com/build/123"),
				CreatedAt:   &github.Timestamp{Time: now},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "status",
			wantTitle: "✅ Status: ci/build",
		},
		// BranchProtectionRuleEvent
		{
			name: "branch_protection_rule created",
			event: &github.BranchProtectionRuleEvent{
				Action: strPtr("created"),
				Rule: &github.BranchProtectionRule{
					ID:        int64Ptr(1000),
					Name:      strPtr("main"),
					CreatedAt: &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "branch_protection_rule",
			wantTitle: "🛡️ Branch Protection Rule created",
		},
		// CodeScanningAlertEvent
		{
			name: "code_scanning_alert created",
			event: &github.CodeScanningAlertEvent{
				Action: strPtr("created"),
				Alert: &github.Alert{
					Number:        intPtr(42),
					RuleID:        strPtr("js/unused-variable"),
					RuleDescription: strPtr("Unused variable"),
					RuleSeverity:  strPtr("warning"),
					HTMLURL:       strPtr("https://github.com/test/repo/security/code-scanning/42"),
					CreatedAt:     &github.Timestamp{Time: now},
				},
				Ref: strPtr("refs/heads/main"),
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "code_scanning_alert",
			wantTitle: "🟡 Code Scanning Alert #42 created",
		},
		// DependabotAlertEvent
		{
			name: "dependabot_alert created",
			event: &github.DependabotAlertEvent{
				Action: strPtr("created"),
				Alert: &github.DependabotAlert{
					Number: intPtr(15),
					Dependency: &github.Dependency{
						Package: &github.VulnerabilityPackage{
							Name: strPtr("lodash"),
						},
					},
					SecurityVulnerability: &github.AdvisoryVulnerability{
						Severity: strPtr("high"),
					},
					HTMLURL:   strPtr("https://github.com/test/repo/security/dependabot/15"),
					CreatedAt: &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "dependabot_alert",
			wantTitle: "🟠 Dependabot Alert #15 created",
		},
		// SecretScanningAlertEvent
		{
			name: "secret_scanning_alert created",
			event: &github.SecretScanningAlertEvent{
				Action: strPtr("created"),
				Alert: &github.SecretScanningAlert{
					Number:              intPtr(8),
					SecretType:          strPtr("github_pat"),
					SecretTypeDisplayName: strPtr("GitHub Personal Access Token"),
					HTMLURL:             strPtr("https://github.com/test/repo/security/secret-scanning/8"),
					CreatedAt:           &github.Timestamp{Time: now},
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "secret_scanning_alert",
			wantTitle: "🔐 Secret Scanning Alert #8 created",
		},
		// TeamAddEvent
		{
			name: "team_add",
			event: &github.TeamAddEvent{
				Team: &github.Team{
					ID:      int64Ptr(200),
					Name:    strPtr("backend-team"),
					HTMLURL: strPtr("https://github.com/orgs/test/teams/backend-team"),
				},
				Repo: &github.Repository{
					FullName: strPtr("test/repo"),
					HTMLURL:  strPtr("https://github.com/test/repo"),
				},
				Sender: &github.User{
					Login:     strPtr("testuser"),
					HTMLURL:   strPtr("https://github.com/testuser"),
					AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
				},
			},
			eventType: "team_add",
			wantTitle: "👥 Team Added: backend-team",
		},
		// PingEvent (ParseEvent doesn't skip, but worker layer will skip)
		{
			name: "ping",
			event: &github.PingEvent{
				Zen:    strPtr("Keep it logically awesome."),
				HookID: int64Ptr(123456),
			},
			eventType: "ping",
			wantTitle: "🔔 GitHub Event: ping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := ParseEvent(tt.event, tt.eventType)

			if tt.wantSkip {
				if !detail.Skip {
					t.Errorf("expected Skip=true, got false")
				}
				return
			}

			if detail.Skip {
				t.Errorf("expected Skip=false, got true")
			}

			if detail.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", detail.Title, tt.wantTitle)
			}

			// Build card to verify it doesn't panic
			card := BuildCard(ctx, "test/repo", "testuser", "https://github.com/testuser", "", detail)
			if card == nil {
				t.Error("BuildCard returned nil")
			}

			// Verify card has content
			cardJSON := card.String()
			if len(cardJSON) < 10 {
				t.Errorf("Card JSON too short: %s", cardJSON)
			}

			fmt.Printf("✓ %s: Title=%q, Text=%q\n", tt.name, detail.Title, detail.Text[:min(50, len(detail.Text))])
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intPtr(i int) *int {
	return &i
}

func TestWorkflowRunAttemptHelpers(t *testing.T) {
	payload := map[string]any{
		"workflow_run": map[string]any{
			"id":          float64(12345),
			"run_attempt": float64(2),
			"triggering_actor": map[string]any{
				"login":      "rerun-user",
				"html_url":   "https://github.com/rerun-user",
				"avatar_url": "https://avatars.githubusercontent.com/u/2",
			},
		},
	}

	if got := workflowRunBaseID(payload); got != "wf:12345" {
		t.Fatalf("workflowRunBaseID() = %q", got)
	}
	if got := workflowRunAttemptID(payload); got != "wf:12345:attempt:2" {
		t.Fatalf("workflowRunAttemptID() = %q", got)
	}

	sender, senderURL, avatarURL := applyWorkflowTriggeringActor(
		payload,
		"github-actions[bot]",
		"https://github.com/apps/github-actions",
		"https://avatars.githubusercontent.com/in/15368",
	)
	if sender != "rerun-user" || senderURL != "https://github.com/rerun-user" || avatarURL != "https://avatars.githubusercontent.com/u/2" {
		t.Fatalf("triggering actor not applied: sender=%q url=%q avatar=%q", sender, senderURL, avatarURL)
	}
}

func TestWorkflowRunAttemptIDDefaultsToBaseID(t *testing.T) {
	payload := map[string]any{
		"workflow_run": map[string]any{
			"id": float64(12345),
		},
	}

	if got := workflowRunAttemptID(payload); got != "wf:12345" {
		t.Fatalf("workflowRunAttemptID() = %q", got)
	}
}

func TestSendNewEventTypeCards(t *testing.T) {
	LoadConfig()
	InitDB()

	if C.Feishu.AppID == "" || C.Feishu.ChatID == "" {
		t.Skip("Feishu credentials not configured")
	}

	ctx := context.Background()
	now := time.Now()

	// Test sending a Discussion card
	detail := ParseEvent(&github.DiscussionEvent{
		Action: strPtr("created"),
		Discussion: &github.Discussion{
			Number:    intPtr(1),
			Title:     strPtr("如何部署这个项目？"),
			Body:      strPtr("我想了解一下部署流程"),
			HTMLURL:   strPtr("https://github.com/test/repo/discussions/1"),
			CreatedAt: &github.Timestamp{Time: now},
		},
		Repo: &github.Repository{
			FullName: strPtr("test/repo"),
			HTMLURL:  strPtr("https://github.com/test/repo"),
		},
		Sender: &github.User{
			Login:     strPtr("testuser"),
			HTMLURL:   strPtr("https://github.com/testuser"),
			AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
		},
	}, "discussion")

	detail.RepoName = "test/repo"
	detail.RepoURL = "https://github.com/test/repo"

	card := BuildCard(ctx, "test/repo", "testuser", "https://github.com/testuser", "", detail)
	msgID, err := SendToChat("", card)
	if err != nil {
		t.Fatalf("send discussion card failed: %v", err)
	}
	fmt.Println("✓ Discussion card sent, message_id:", msgID)

	// Test sending a Deployment Status card
	detail2 := ParseEvent(&github.DeploymentStatusEvent{
		Action: strPtr("created"),
		Deployment: &github.Deployment{
			Environment: strPtr("production"),
		},
		DeploymentStatus: &github.DeploymentStatus{
			State:       strPtr("success"),
			Description: strPtr("部署成功"),
			TargetURL:   strPtr("https://example.com"),
			CreatedAt:   &github.Timestamp{Time: now},
		},
		Repo: &github.Repository{
			FullName: strPtr("test/repo"),
			HTMLURL:  strPtr("https://github.com/test/repo"),
		},
		Sender: &github.User{
			Login:     strPtr("testuser"),
			HTMLURL:   strPtr("https://github.com/testuser"),
			AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
		},
	}, "deployment_status")

	detail2.RepoName = "test/repo"
	detail2.RepoURL = "https://github.com/test/repo"

	card2 := BuildCard(ctx, "test/repo", "testuser", "https://github.com/testuser", "", detail2)
	msgID2, err := SendToChat("", card2)
	if err != nil {
		t.Fatalf("send deployment status card failed: %v", err)
	}
	fmt.Println("✓ Deployment Status card sent, message_id:", msgID2)

	// Test sending a Code Scanning Alert card
	detail3 := ParseEvent(&github.CodeScanningAlertEvent{
		Action: strPtr("created"),
		Alert: &github.Alert{
			Number:          intPtr(42),
			RuleDescription: strPtr("未使用的变量"),
			RuleSeverity:    strPtr("warning"),
			HTMLURL:         strPtr("https://github.com/test/repo/security/code-scanning/42"),
			CreatedAt:       &github.Timestamp{Time: now},
		},
		Ref: strPtr("refs/heads/main"),
		Repo: &github.Repository{
			FullName: strPtr("test/repo"),
			HTMLURL:  strPtr("https://github.com/test/repo"),
		},
		Sender: &github.User{
			Login:     strPtr("testuser"),
			HTMLURL:   strPtr("https://github.com/testuser"),
			AvatarURL: strPtr("https://avatars.githubusercontent.com/u/12345"),
		},
	}, "code_scanning_alert")

	detail3.RepoName = "test/repo"
	detail3.RepoURL = "https://github.com/test/repo"

	card3 := BuildCard(ctx, "test/repo", "testuser", "https://github.com/testuser", "", detail3)
	msgID3, err := SendToChat("", card3)
	if err != nil {
		t.Fatalf("send code scanning alert card failed: %v", err)
	}
	fmt.Println("✓ Code Scanning Alert card sent, message_id:", msgID3)
}
