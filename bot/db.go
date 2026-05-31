package bot

import (
	"context"
	"database/sql"
	"embed"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/migrate"
)

//go:embed migrations/*.sql
var sqlMigrations embed.FS

var DB *bun.DB

// MessageRecord 消息记录表，用于追踪 GitHub 事件与飞书消息的对应关系
type MessageRecord struct {
	bun.BaseModel `bun:"table:message_records,alias:mr"`

	ID              uint64    `bun:",pk,autoincrement"`
	GithubID        string    `bun:",unique,notnull"` // 可能是 workflow_run_id、分支引用或 commit SHA
	FeishuMessageID string    `bun:",notnull"`
	ChatID          string    `bun:",notnull"`
	RepoName        string    `bun:",notnull"`
	Ref             string    `bun:""`
	EventType       string    `bun:",notnull"`
	Content         string    `bun:"type:text"` // 存储卡片详情的 JSON
	CardString      string    `bun:"type:text"` // 存储卡片内容的字符串表示，用于去重
	CreatedAt       time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	DeletedAt       time.Time `bun:",soft_delete,nullzero"`

	// 关联原始事件 (新增)
	EventID uint64 `bun:",notnull"`

	// push/create 事件：记录 head commit SHA，用于 CI/tag 精确关联
	HeadSHA string `bun:""`

	// 新增：图片状态，用于后台刷新
	ImageStatus string `bun:",default:'done'"` // done, pending
	AvatarURL   string `bun:""`                // 原始头像 URL

	// Workflow 专用：记录开始运行时间，用于超时提醒
	WorkflowStartedAt time.Time `bun:",nullzero"`
	// Workflow 专用：是否已经发送过超时提醒
	TimeoutNotified bool `bun:",default:false"`

	// 消息类型标识，用于区分正常消息和删除消息（支持合并查询）
	RecordType string `bun:",default:'normal'"` // normal, deleted

	// CI 事件：关联的父消息 ID（push/PR），用于内联 CI 状态到父消息卡片
	ParentMsgID string `bun:""`

	// 发送者元数据，用于重建卡片时保持一致
	Sender     string `bun:""`
	SenderURL  string `bun:""`
	AvatarURL2 string `bun:""` // 原始发送者头像 URL（与 AvatarURL 区分，AvatarURL 用于图片刷新）
}

// WebhookEvent 存储所有来自 GitHub 的原始请求，持久化保存
type WebhookEvent struct {
	bun.BaseModel `bun:"table:webhook_events,alias:we"`

	ID             uint64    `bun:",pk,autoincrement"`
	DeliveryID     string    `bun:",unique"`            // X-GitHub-Delivery 标头，用于幂等性检查
	EventType      string    `bun:",notnull"`           // X-GitHub-Event 标头
	HookID         int64     `bun:""`                   // X-GitHub-Hook-ID 标头
	Payload        string    `bun:"type:text"`          // 原始 Webhook 负载
	Status         string    `bun:",default:'pending'"` // pending, processed, failed
	RetryCount     int       `bun:",default:0"`
	RescheduleCount int      `bun:",default:0"` // CI 事件等待 push 事件的重调度次数
	HeadSHA         string   `bun:""`           // 从 payload 中提取的 head SHA，用于快速 CI 重调度查找
	Ref             string   `bun:""`           // 分支/标签引用，用于 CI 重调度时关联 create 事件
	CreatedAt      time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

// ImageCache 图片缓存表，加速头像显示
type ImageCache struct {
	bun.BaseModel `bun:"table:image_caches,alias:ic"`

	URL       string    `bun:",pk"`
	ImgKey    string    `bun:",notnull"`
	Hash      string    `bun:",nullzero"` // 图片内容的哈希值 (MD5)
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

// InitDB 初始化数据库连接并执行 SQL 迁移
func InitDB() {
	if C.Database.URL == "" {
		slog.Warn("Skipping database initialization: DATABASE_URL not set")
		return
	}

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(C.Database.URL)))
	db := bun.NewDB(sqldb, pgdialect.New())
	DB = db

	// 运行 SQL 迁移（建表 + 加列，全部由 migrations/ 目录管理）
	ctx := context.Background()
	migrations := migrate.NewMigrations()
	if err := migrations.Discover(sqlMigrations); err != nil {
		slog.Error("Failed to discover SQL migrations", "error", err)
		return
	}
	migrator := migrate.NewMigrator(db, migrations)
	if err := migrator.Init(ctx); err != nil {
		slog.Error("Failed to init migration tables", "error", err)
		return
	}
	group, err := migrator.Migrate(ctx)
	if err != nil {
		slog.Error("SQL migration failed", "error", err)
		return
	}
	if group.IsZero() {
		slog.Info("SQL migrations up to date")
	} else {
		slog.Info("SQL migrations applied", "group", group.ID)
	}
	slog.Info("Database initialization successful")
}
