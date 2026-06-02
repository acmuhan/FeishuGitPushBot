package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRepositoryLifecycle 模拟完整的仓库生命周期
func TestRepositoryLifecycle(t *testing.T) {
	LoadConfig()
	InitDB()

	if C.Feishu.AppID == "" || C.Feishu.ChatID == "" {
		t.Skip("Feishu credentials not configured")
	}

	// 启动消息处理 Worker
	if DB != nil {
		go messageWorker()
		time.Sleep(500 * time.Millisecond) // 等待 worker 启动
	}

	secret := C.Github.Key
	router := InitRouter()
	ts := httptest.NewServer(router)
	defer ts.Close()

	repo := "NCUHOME/FeishuGitPushBot"
	repoURL := "https://github.com/" + repo
	baseSHA := "abc1234567890def"

	// 定义完整的生命周期事件序列
	events := []struct {
		name      string
		eventType string
		delay     time.Duration
		payload   map[string]interface{}
	}{
		// ========== 1. 创建仓库 ==========
		{
			name:      "创建仓库",
			eventType: "repository",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "created",
				"repository": map[string]interface{}{
					"full_name":  repo,
					"html_url":   repoURL,
					"description": "飞书 GitHub Webhook 机器人",
					"created_at": time.Now().Format(time.RFC3339),
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 2. 添加团队到仓库 ==========
		{
			name:      "添加团队",
			eventType: "team_add",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"team": map[string]interface{}{
					"id":       1001,
					"name":     "backend-team",
					"html_url": "https://github.com/orgs/NCUHOME/teams/backend-team",
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "admin",
					"html_url":   "https://github.com/admin",
					"avatar_url": "https://avatars.githubusercontent.com/u/99999",
				},
			},
		},

		// ========== 3. 添加成员 ==========
		{
			name:      "添加成员",
			eventType: "member",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "added",
				"member": map[string]interface{}{
					"login":      "developer1",
					"html_url":   "https://github.com/developer1",
					"avatar_url": "https://avatars.githubusercontent.com/u/11111",
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 4. Push 初始代码 ==========
		{
			name:      "Push 初始代码",
			eventType: "push",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"ref": "refs/heads/main",
				"after": baseSHA,
				"head_commit": map[string]interface{}{
					"id":      baseSHA,
					"message": "feat: 初始化项目结构\n\n- 添加 main.go 入口\n- 配置 Go modules\n- 添加 README",
					"url":     repoURL + "/commit/" + baseSHA,
					"timestamp": time.Now().Format(time.RFC3339),
					"author": map[string]interface{}{
						"login": "hangone",
						"name":  "Hangone",
					},
				},
				"commits": []interface{}{
					map[string]interface{}{
						"id":      baseSHA,
						"message": "feat: 初始化项目结构",
						"url":     repoURL + "/commit/" + baseSHA,
						"author": map[string]interface{}{
							"login": "hangone",
							"name":  "Hangone",
						},
					},
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 5. 创建 PR ==========
		{
			name:      "创建 PR",
			eventType: "pull_request",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "opened",
				"number": 1,
				"pull_request": map[string]interface{}{
					"number": 1,
					"title":  "feat: 添加 Webhook 事件处理",
					"body":   "## 变更说明\n\n- 新增 18 种 GitHub 事件类型支持\n- 实现通用去重机制\n- 优化飞书卡片展示\n\n## 测试\n\n- [x] 单元测试\n- [x] 集成测试",
					"html_url": repoURL + "/pull/1",
					"state":    "open",
					"head": map[string]interface{}{
						"ref": "feature/webhook",
						"sha": "def456789abc",
						"repo": map[string]interface{}{
							"full_name": repo,
							"html_url":  repoURL,
						},
					},
					"base": map[string]interface{}{
						"ref": "main",
					},
					"user": map[string]interface{}{
						"login":      "developer1",
						"html_url":   "https://github.com/developer1",
						"avatar_url": "https://avatars.githubusercontent.com/u/11111",
					},
					"created_at": time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "developer1",
					"html_url":   "https://github.com/developer1",
					"avatar_url": "https://avatars.githubusercontent.com/u/11111",
				},
			},
		},

		// ========== 6. PR Review ==========
		{
			name:      "PR Review",
			eventType: "pull_request_review",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "submitted",
				"review": map[string]interface{}{
					"body":    "LGTM! 代码质量很好，测试覆盖全面。",
					"state":   "approved",
					"html_url": repoURL + "/pull/1#pullrequestreview-100",
					"submitted_at": time.Now().Format(time.RFC3339),
					"user": map[string]interface{}{
						"login": "hangone",
					},
				},
				"pull_request": map[string]interface{}{
					"number": 1,
					"title":  "feat: 添加 Webhook 事件处理",
					"html_url": repoURL + "/pull/1",
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 7. Merge PR ==========
		{
			name:      "Merge PR",
			eventType: "pull_request",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "closed",
				"number": 1,
				"pull_request": map[string]interface{}{
					"number":  1,
					"title":   "feat: 添加 Webhook 事件处理",
					"html_url": repoURL + "/pull/1",
					"state":   "closed",
					"merged":  true,
					"merged_at": time.Now().Format(time.RFC3339),
					"merged_by": map[string]interface{}{
						"login": "hangone",
					},
					"head": map[string]interface{}{
						"sha": "def456789abc",
					},
					"base": map[string]interface{}{
						"ref": "main",
					},
					"user": map[string]interface{}{
						"login":      "developer1",
						"html_url":   "https://github.com/developer1",
						"avatar_url": "https://avatars.githubusercontent.com/u/11111",
					},
					"created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 8. 创建 Tag ==========
		{
			name:      "创建 Tag v1.0.0",
			eventType: "create",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"ref":          "v1.0.0",
				"ref_type":     "tag",
				"master_branch": "main",
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 9. Workflow 开始 - workflow_run in_progress ==========
		{
			name:      "Workflow 开始运行",
			eventType: "workflow_run",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "requested",
				"workflow_run": map[string]interface{}{
					"id":          100001,
					"name":        "CI/CD Pipeline",
					"head_branch": "main",
					"head_sha":    baseSHA,
					"run_number":  1,
					"event":       "push",
					"status":      "in_progress",
					"conclusion":  nil,
					"html_url":    repoURL + "/actions/runs/100001",
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
					"run_started_at": time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 10. Job 1: Checkout - queued → in_progress → completed ==========
		{
			name:      "Job: Checkout 开始",
			eventType: "workflow_job",
			delay:     300 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "queued",
				"workflow_job": map[string]interface{}{
					"id":          200001,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Checkout",
					"status":      "queued",
					"conclusion":  nil,
					"head_branch": "main",
					"head_sha":    baseSHA,
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},
		{
			name:      "Job: Checkout 运行中",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "in_progress",
				"workflow_job": map[string]interface{}{
					"id":          200001,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Checkout",
					"status":      "in_progress",
					"conclusion":  nil,
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},
		{
			name:      "Job: Checkout 完成",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "completed",
				"workflow_job": map[string]interface{}{
					"id":          200001,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Checkout",
					"status":      "completed",
					"conclusion":  "success",
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Add(-5 * time.Second).Format(time.RFC3339),
					"completed_at": time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Add(-10 * time.Second).Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 11. Job 2: Test - 运行中 ==========
		{
			name:      "Job: Test 开始",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "in_progress",
				"workflow_job": map[string]interface{}{
					"id":          200002,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Test",
					"status":      "in_progress",
					"conclusion":  nil,
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 12. Job 2: Test 完成 ==========
		{
			name:      "Job: Test 完成",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "completed",
				"workflow_job": map[string]interface{}{
					"id":          200002,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Test",
					"status":      "completed",
					"conclusion":  "success",
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Add(-15 * time.Second).Format(time.RFC3339),
					"completed_at": time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Add(-20 * time.Second).Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 13. Job 3: Build - 运行中 ==========
		{
			name:      "Job: Build 开始",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "in_progress",
				"workflow_job": map[string]interface{}{
					"id":          200003,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Build",
					"status":      "in_progress",
					"conclusion":  nil,
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 14. Job 3: Build 完成 ==========
		{
			name:      "Job: Build 完成",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "completed",
				"workflow_job": map[string]interface{}{
					"id":          200003,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Build",
					"status":      "completed",
					"conclusion":  "success",
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Add(-25 * time.Second).Format(time.RFC3339),
					"completed_at": time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Add(-30 * time.Second).Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 15. Job 4: Deploy - 运行中 ==========
		{
			name:      "Job: Deploy 开始",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "in_progress",
				"workflow_job": map[string]interface{}{
					"id":          200004,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Deploy",
					"status":      "in_progress",
					"conclusion":  nil,
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 16. Job 4: Deploy 完成 ==========
		{
			name:      "Job: Deploy 完成",
			eventType: "workflow_job",
			delay:     200 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "completed",
				"workflow_job": map[string]interface{}{
					"id":          200004,
					"run_id":      100001,
					"workflow_name": "CI/CD Pipeline",
					"name":        "Deploy",
					"status":      "completed",
					"conclusion":  "success",
					"head_branch": "main",
					"head_sha":    baseSHA,
					"started_at":  time.Now().Add(-35 * time.Second).Format(time.RFC3339),
					"completed_at": time.Now().Format(time.RFC3339),
					"created_at":  time.Now().Add(-40 * time.Second).Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 17. Workflow 完成 ==========
		{
			name:      "Workflow 完成",
			eventType: "workflow_run",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "completed",
				"workflow_run": map[string]interface{}{
					"id":          100001,
					"name":        "CI/CD Pipeline",
					"head_branch": "main",
					"head_sha":    baseSHA,
					"run_number":  1,
					"event":       "push",
					"status":      "completed",
					"conclusion":  "success",
					"html_url":    repoURL + "/actions/runs/100001",
					"created_at":  time.Now().Add(-45 * time.Second).Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
					"run_started_at": time.Now().Add(-45 * time.Second).Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},

		// ========== 18. 发布 Release ==========
		{
			name:      "发布 Release v1.0.0",
			eventType: "release",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "published",
				"release": map[string]interface{}{
					"id":          50001,
					"tag_name":    "v1.0.0",
					"name":        "v1.0.0 - 首个正式版本",
					"body":        "## 🎉 v1.0.0 正式发布\n\n### ✨ 新功能\n\n- 支持 18 种 GitHub Webhook 事件类型\n- 飞书卡片消息展示\n- 事件去重与合并机制\n- CI/CD 状态内联显示\n\n### 🔧 改进\n\n- 优化卡片布局和样式\n- 支持 commit 多作者显示\n- 支持 Co-authored-by 解析\n\n### 🐛 修复\n\n- 修复 workflow_job 覆盖问题\n- 修复评论 ID 冲突问题",
					"html_url":    repoURL + "/releases/tag/v1.0.0",
					"created_at":  time.Now().Format(time.RFC3339),
					"published_at": time.Now().Format(time.RFC3339),
					"author": map[string]interface{}{
						"login":      "hangone",
						"html_url":   "https://github.com/hangone",
						"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
					},
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 19. 删除仓库 ==========
		{
			name:      "删除仓库",
			eventType: "repository",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "deleted",
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},

		// ========== 20. 移除团队 ==========
		{
			name:      "移除团队",
			eventType: "team_add",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "removed",
				"team": map[string]interface{}{
					"id":       1001,
					"name":     "backend-team",
					"html_url": "https://github.com/orgs/NCUHOME/teams/backend-team",
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "admin",
					"html_url":   "https://github.com/admin",
					"avatar_url": "https://avatars.githubusercontent.com/u/99999",
				},
			},
		},

		// ========== 21. 移除成员 ==========
		{
			name:      "移除成员",
			eventType: "member",
			delay:     500 * time.Millisecond,
			payload: map[string]interface{}{
				"action": "removed",
				"member": map[string]interface{}{
					"login":      "developer1",
					"html_url":   "https://github.com/developer1",
					"avatar_url": "https://avatars.githubusercontent.com/u/11111",
				},
				"repository": map[string]interface{}{
					"full_name": repo,
					"html_url":  repoURL,
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},
	}

	fmt.Println("🚀 开始模拟完整仓库生命周期...")
	fmt.Println(strings.Repeat("=", 60))

	successCount := 0
	failCount := 0

	for i, tc := range events {
		t.Run(tc.name, func(t *testing.T) {
			// 等待指定延迟
			if tc.delay > 0 {
				time.Sleep(tc.delay)
			}

			// 序列化 payload
			payload, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
			}

			// 生成唯一的 delivery ID
			deliveryID := fmt.Sprintf("lifecycle-%s-%d", tc.eventType, time.Now().UnixNano())

			// 创建请求
			req, err := http.NewRequest("POST", ts.URL+"/github/webhook", strings.NewReader(string(payload)))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", tc.eventType)
			req.Header.Set("X-GitHub-Delivery", deliveryID)

			// 如果配置了 secret，添加签名
			if secret != "" {
				signature := generateSignature(secret, payload)
				req.Header.Set("X-Hub-Signature-256", signature)
			}

			// 发送请求
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// 检查响应
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d: %v", resp.StatusCode, result)
				failCount++
				return
			}

			code, ok := result["code"].(float64)
			if !ok || code != 0 {
				t.Errorf("Expected code 0, got %v", result)
				failCount++
				return
			}

			msg, _ := result["msg"].(string)
			fmt.Printf("[%02d] ✓ %s (%s)\n", i+1, tc.name, msg)
			successCount++
		})
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("🏁 生命周期模拟完成: %d/%d 成功\n", successCount, len(events))
	if failCount > 0 {
		fmt.Printf("❌ 失败: %d\n", failCount)
	}

	// 等待 Worker 处理完所有消息
	fmt.Println("\n⏳ 等待 Worker 处理消息队列...")
	time.Sleep(10 * time.Second)

	// 查询并显示消息关联关系
	if DB != nil {
		var records []MessageRecord
		DB.NewSelect().Model(&records).
			Where("repo_name = ?", repo).
			Order("id ASC").
			Scan(context.Background())

		fmt.Println("\n📊 消息关联关系:")
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("%-15s %-45s %-10s %s\n", "Event", "Github ID", "Reply?", "Message ID")
		fmt.Println(strings.Repeat("-", 80))

		rootCount := 0
		replyCount := 0
		for _, r := range records {
			isReply := "─"
			if r.ParentMsgID != "" {
				isReply = "✓"
				replyCount++
			} else {
				rootCount++
			}
			fmt.Printf("%-15s %-45s %-10s %s\n",
				r.EventType,
				r.GithubID[:min(45, len(r.GithubID))],
				isReply,
				r.FeishuMessageID[:min(25, len(r.FeishuMessageID))])
		}
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("总计: %d 条消息 (%d 条根消息, %d 条话题回复)\n",
			len(records), rootCount, replyCount)
	}
}
