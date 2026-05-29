package bot

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSendDeleteCard(t *testing.T) {
	LoadConfig()
	InitDB()

	detail := EventDetail{
		Title:     "🗑️ Tag Deleted: v1.2",
		RefName:   "v1.2",
		RefURL:    "https://github.com/NCUHOME/K8sSetImageAction/tags",
		IsTag:     true,
		IsDeleted: true,
		EventTime: time.Now().Format(time.RFC3339),
		RepoName:  "NCUHOME/K8sSetImageAction",
		RepoURL:   "https://github.com/NCUHOME/K8sSetImageAction",
		URL:       "https://github.com/NCUHOME/K8sSetImageAction/tags",
	}

	ctx := context.Background()
	card := BuildCard(ctx, "NCUHOME/K8sSetImageAction", "HakimYu", "https://github.com/HakimYu", "", detail)

	msgID, err := SendToChat("", card)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	fmt.Println("sent message_id:", msgID)
}

func TestOnConflictInsert(t *testing.T) {
	LoadConfig()
	InitDB()
	if DB == nil {
		t.Skip("DB not initialized")
	}

	ctx := context.Background()
	githubID := "test:conflict:" + fmt.Sprintf("%d", time.Now().UnixNano())

	// First insert
	_, err := DB.NewInsert().Model(&MessageRecord{
		GithubID:      githubID,
		FeishuMessageID: "msg_001",
		ChatID:        "test_chat",
		RepoName:      "test/repo",
		EventType:     "push",
		Content:       "{}",
		EventID:       99999,
		HeadSHA:       "sha_original",
	}).On("CONFLICT (github_id) DO UPDATE").
		Set("feishu_message_id = EXCLUDED.feishu_message_id").
		Set("head_sha = EXCLUDED.head_sha").
		Exec(ctx)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	fmt.Println("first insert OK")

	// Second insert (upsert)
	_, err = DB.NewInsert().Model(&MessageRecord{
		GithubID:      githubID,
		FeishuMessageID: "msg_002",
		ChatID:        "test_chat",
		RepoName:      "test/repo",
		EventType:     "push",
		Content:       `{"updated":true}`,
		EventID:       99998,
		HeadSHA:       "sha_updated",
	}).On("CONFLICT (github_id) DO UPDATE").
		Set("feishu_message_id = EXCLUDED.feishu_message_id").
		Set("head_sha = EXCLUDED.head_sha").
		Exec(ctx)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	fmt.Println("upsert OK")

	// Verify
	var rec MessageRecord
	err = DB.NewSelect().Model(&rec).Where("github_id = ?", githubID).Scan(ctx)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	fmt.Printf("feishu_message_id=%s, head_sha=%s\n", rec.FeishuMessageID, rec.HeadSHA)
	if rec.FeishuMessageID != "msg_002" {
		t.Errorf("expected msg_002, got %s", rec.FeishuMessageID)
	}
	if rec.HeadSHA != "sha_updated" {
		t.Errorf("expected sha_updated, got %s", rec.HeadSHA)
	}

	// Cleanup
	_, _ = DB.NewDelete().Model(&rec).Where("github_id = ?", githubID).Exec(ctx)
}
