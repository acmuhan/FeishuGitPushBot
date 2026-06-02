package bot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// generateSignature 生成 GitHub webhook 签名
func generateSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestWebhookFlow 模拟完整的 GitHub Webhook 处理流程
func TestWebhookFlow(t *testing.T) {
	LoadConfig()
	// 不初始化数据库，使用兜底模式直接发送

	if C.Feishu.AppID == "" || C.Feishu.ChatID == "" {
		t.Skip("Feishu credentials not configured")
	}

	secret := C.Github.Key

	testCases := []struct {
		name      string
		eventType string
		payload   map[string]interface{}
	}{
		{
			name:      "Discussion 创建",
			eventType: "discussion",
			payload: map[string]interface{}{
				"action": "created",
				"discussion": map[string]interface{}{
					"id":         123456,
					"number":     42,
					"title":      "如何部署这个项目到生产环境？",
					"body":       "我想了解一下完整的部署流程，包括 CI/CD 配置、容器化方案等。\n\n## 背景\n\n我们团队需要将这个项目部署到 Kubernetes 集群，希望了解最佳实践。",
					"html_url":   "https://github.com/NCUHOME/FeishuGitPushBot/discussions/42",
					"created_at": time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},
		{
			name:      "Deployment Status 成功",
			eventType: "deployment_status",
			payload: map[string]interface{}{
				"action": "created",
				"deployment": map[string]interface{}{
					"id":          789,
					"ref":         "v2.1.0",
					"environment": "production",
					"description": "Release v2.1.0",
				},
				"deployment_status": map[string]interface{}{
					"id":          456,
					"state":       "success",
					"description": "Deployment completed successfully",
					"target_url":  "https://app.example.com",
					"created_at":  time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},
		{
			name:      "Code Scanning Alert 错误级别",
			eventType: "code_scanning_alert",
			payload: map[string]interface{}{
				"action": "created",
				"alert": map[string]interface{}{
					"number": 15,
					"rule": map[string]interface{}{
						"id":          "js/sql-injection",
						"description": "SQL injection vulnerability",
						"severity":    "error",
					},
					"rule_id":          "js/sql-injection",
					"rule_description": "SQL injection vulnerability",
					"rule_severity":    "error",
					"html_url":         "https://github.com/NCUHOME/FeishuGitPushBot/security/code-scanning/15",
					"created_at":       time.Now().Format(time.RFC3339),
					"state":            "open",
				},
				"ref": "refs/heads/main",
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "github-code-scanning[bot]",
					"html_url":   "https://github.com/apps/github-code-scanning",
					"avatar_url": "https://avatars.githubusercontent.com/in/96389",
				},
			},
		},
		{
			name:      "Dependabot Alert 高危",
			eventType: "dependabot_alert",
			payload: map[string]interface{}{
				"action": "created",
				"alert": map[string]interface{}{
					"number": 28,
					"dependency": map[string]interface{}{
						"package": map[string]interface{}{
							"name":      "lodash",
							"ecosystem": "npm",
						},
					},
					"security_advisory": map[string]interface{}{
						"summary":  "Prototype Pollution in lodash",
						"severity": "high",
						"ghsa_id":  "GHSA-jf85-cpcp-j695",
					},
					"security_vulnerability": map[string]interface{}{
						"severity": "high",
					},
					"html_url":   "https://github.com/NCUHOME/FeishuGitPushBot/security/dependabot/28",
					"created_at": time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "dependabot[bot]",
					"html_url":   "https://github.com/apps/dependabot",
					"avatar_url": "https://avatars.githubusercontent.com/in/29110",
				},
			},
		},
		{
			name:      "Secret Scanning Alert",
			eventType: "secret_scanning_alert",
			payload: map[string]interface{}{
				"action": "created",
				"alert": map[string]interface{}{
					"number":                 5,
					"secret_type":            "github_pat",
					"secret_type_display_name": "GitHub Personal Access Token",
					"html_url":               "https://github.com/NCUHOME/FeishuGitPushBot/security/secret-scanning/5",
					"created_at":             time.Now().Format(time.RFC3339),
					"state":                  "open",
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "github-security[bot]",
					"html_url":   "https://github.com/apps/github-security",
					"avatar_url": "https://avatars.githubusercontent.com/in/99999",
				},
			},
		},
		{
			name:      "Milestone 创建",
			eventType: "milestone",
			payload: map[string]interface{}{
				"action": "created",
				"milestone": map[string]interface{}{
					"id":            100,
					"number":        3,
					"title":         "v3.0.0 - 重大版本更新",
					"description":   "包含多项新功能和性能优化\n- 支持 18 种新事件类型\n- 通用去重机制\n- 部署状态追踪",
					"state":         "open",
					"open_issues":   12,
					"closed_issues": 8,
					"html_url":      "https://github.com/NCUHOME/FeishuGitPushBot/milestone/3",
					"created_at":    time.Now().Format(time.RFC3339),
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},
		{
			name:      "Label 创建",
			eventType: "label",
			payload: map[string]interface{}{
				"action": "created",
				"label": map[string]interface{}{
					"id":          500,
					"name":        "security",
					"description": "安全相关的变更",
					"color":       "e11d48",
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},
		{
			name:      "Branch Protection Rule 更新",
			eventType: "branch_protection_rule",
			payload: map[string]interface{}{
				"action": "edited",
				"rule": map[string]interface{}{
					"id":   200,
					"name": "main",
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "hangone",
					"html_url":   "https://github.com/hangone",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345678",
				},
			},
		},
		{
			name:      "Status Event (CI)",
			eventType: "status",
			payload: map[string]interface{}{
				"sha":         "abc123def456789",
				"state":       "success",
				"context":     "ci/build",
				"description": "Build passed",
				"target_url":  "https://github.com/NCUHOME/FeishuGitPushBot/actions/runs/123456",
				"created_at":  time.Now().Format(time.RFC3339),
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "github-actions[bot]",
					"html_url":   "https://github.com/apps/github-actions",
					"avatar_url": "https://avatars.githubusercontent.com/in/15368",
				},
			},
		},
		{
			name:      "Team Added to Repo",
			eventType: "team_add",
			payload: map[string]interface{}{
				"team": map[string]interface{}{
					"id":        300,
					"name":      "backend-team",
					"html_url":  "https://github.com/orgs/NCUHOME/teams/backend-team",
				},
				"repository": map[string]interface{}{
					"full_name": "NCUHOME/FeishuGitPushBot",
					"html_url":  "https://github.com/NCUHOME/FeishuGitPushBot",
				},
				"sender": map[string]interface{}{
					"login":      "admin-user",
					"html_url":   "https://github.com/admin-user",
					"avatar_url": "https://avatars.githubusercontent.com/u/99999",
				},
			},
		},
	}

	// 创建测试 HTTP 服务器
	router := InitRouter()
	ts := httptest.NewServer(router)
	defer ts.Close()

	successCount := 0
	failCount := 0

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 序列化 payload
			payload, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
			}

			// 生成唯一的 delivery ID
			deliveryID := fmt.Sprintf("test-%s-%d", tc.eventType, time.Now().UnixNano())

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
			fmt.Printf("✓ %s: %s (delivery: %s)\n", tc.name, msg, deliveryID)
			successCount++

			// 等待消息发送
			time.Sleep(1 * time.Second)
		})
	}

	fmt.Printf("\n========== 测试结果 ==========\n")
	fmt.Printf("成功: %d / %d\n", successCount, len(testCases))
	fmt.Printf("失败: %d / %d\n", failCount, len(testCases))
}
