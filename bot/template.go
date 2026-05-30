package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/kyokomi/emoji/v2"
)

type EventDetail struct {
	Title         string   `json:"title"`
	Text          string   `json:"text"`
	URL           string   `json:"url"`
	Ref           string   `json:"ref"`      // 原始 Ref markdown
	RefName       string   `json:"ref_name"` // 纯文本引用名 (如 main)
	RefURL        string   `json:"ref_url"`  // 引用的 URL
	ReplyToTitle  string   `json:"reply_to_title"`
	FoldableBody  string   `json:"foldable_body"`
	Skip          bool     `json:"skip"`
	SHA           string   `json:"sha"`
	FullSHA       string   `json:"full_sha"` // 完整 SHA，用于构建 commit 链接
	IsTag         bool     `json:"is_tag"`
	IsDeleted     bool     `json:"is_deleted"` // 分支/标签删除事件
	AuthorAvatars []string `json:"author_avatars"` // 提交者或协作者的头像 URL 列表
	AuthorLogins  []string `json:"author_logins"`  // 提交者或协作者的 login 列表（与 AuthorAvatars 顺序对应）
	Action        string   `json:"action"`         // 事件具体动作
	ExtraReply    string   `json:"extra_reply"`    // 需要另起一段话题回复的内容
	EventTime     string   `json:"event_time"`     // GitHub 事件原始发生时间 (RFC3339)
	RepoName      string   `json:"repo_name"`      // 仓库全名 (如 NCUHOME/repo)，用于合并展示
	RepoURL       string   `json:"repo_url"`       // 仓库 HTML URL，用于 BuildCard 构建链接

	// CI 状态：内联到触发事件的卡片中
	CIStatuses []CIStatus `json:"ci_statuses,omitempty"`
	// 合并事件：记录时间范围和数量
	EventTimeEnd string `json:"event_time_end,omitempty"` // 合并事件的最后时间
	EventCount   int    `json:"event_count,omitempty"`     // 合并事件中的条目数（按 commit 计）
	CommitCount  int    `json:"commit_count,omitempty"`    // push 事件的 commit 总数（用于折叠）
}

// CIStatus 单条 CI/Workflow 运行状态
type CIStatus struct {
	WorkflowName string `json:"workflow_name"` // workflow 名称
	Status       string `json:"status"`        // completed / in_progress / queued / ...
	Conclusion   string `json:"conclusion"`    // success / failure / cancelled / ...
	RunID        int64  `json:"run_id"`        // GitHub workflow run ID，用于构建链接
	Ref          string `json:"ref"`           // 分支名
	Duration     string `json:"duration"`      // 格式化后的耗时
	UpdatedAt    string `json:"updated_at"`    // 最后更新时间 (RFC3339)
	// Job 专用字段
	JobName    string `json:"job_name,omitempty"`    // job 名称（用于显示）
	ParentRunID int64  `json:"parent_run_id,omitempty"` // 关联的 workflow run ID
}

// ParseEvent 解析 GitHub 事件为极简明了的 Detail
func ParseEvent(event any, eventType string) EventDetail {
	d := EventDetail{
		Title: fmt.Sprintf("🔔 GitHub Event: %s", eventType),
		Skip:  false, // 默认不跳过任何事件
	}

	switch e := event.(type) {
	case *github.PushEvent:
		ref := e.GetRef()
		// 更鲁棒的标签检测：检查 refs/tags/ 前缀或 ref 本身
		isTag := strings.HasPrefix(ref, "refs/tags/")
		refShort := ""
		if isTag {
			refShort = strings.TrimPrefix(ref, "refs/tags/")
			d.IsTag = true
		} else {
			refShort = strings.TrimPrefix(ref, "refs/heads/")
		}
		repoUrl := ""
		if repo := e.GetRepo(); repo != nil {
			repoUrl = repo.GetHTMLURL()
		}

		// 跳过 tag 的创建和删除事件，由 CreateEvent/DeleteEvent 处理，避免重复推送
		if isTag && (e.GetCreated() || e.GetDeleted()) {
			d.Skip = true
			return d
		}

		if isTag {
			d.Title = fmt.Sprintf("🏷️ Tag: %s", refShort)
			d.RefName = refShort
			d.RefURL = fmt.Sprintf("%s/releases/tag/%s", repoUrl, refShort)
			d.URL = d.RefURL
		} else if strings.HasPrefix(ref, "refs/heads/") {
			d.Title = "🍏 New commits"
			d.RefName = refShort
			d.RefURL = fmt.Sprintf("%s/tree/%s", repoUrl, refShort)
		}

		if len(e.Commits) > 0 {
			authors := make(map[string]bool)
			avatarMap := make(map[string]string)
			for _, c := range e.Commits {
				login := ""
				if author := c.GetAuthor(); author != nil {
					login = author.GetLogin()
				}
				if login == "" {
					if committer := c.GetCommitter(); committer != nil {
						login = committer.GetLogin()
					}
				}
				if login != "" {
					authors[login] = true
					avatarMap[login] = fmt.Sprintf("https://avatars.githubusercontent.com/%s", login)
				}
				// 检查 Co-authored-by
				for _, coAuthor := range parseCoAuthors(c.GetMessage()) {
					if coAuthor.Avatar != "" {
						key := coAuthor.Login
						if key == "" {
							key = coAuthor.Name
						}
						authors[key] = true
						avatarMap[key] = coAuthor.Avatar
					}
				}
			}
			multiAuthor := len(authors) > 1

			// 收集所有头像和 login（保持顺序一致）
			for login, url := range avatarMap {
				d.AuthorAvatars = append(d.AuthorAvatars, url)
				d.AuthorLogins = append(d.AuthorLogins, login)
			}

			var lines []string
			for i, c := range e.Commits {
				emojiIcon := "🔸"
				if i%2 != 0 {
					emojiIcon = "🔹"
				}

				msg := SafeText(c.GetMessage(), 5000)
				msg = ProcessCommitMessage(msg, repoUrl)

				shortSHA := ""
				if sha := c.GetID(); sha != "" {
					if len(sha) > 7 {
						shortSHA = sha[:7]
					} else {
						shortSHA = sha
					}
				}

				hashPart := ""
				if shortSHA != "" && c.GetURL() != "" {
					hashPart = fmt.Sprintf(" ([%s](%s))", shortSHA, c.GetURL())
				}

				authorPart := ""
				if multiAuthor {
					login := ""
					name := ""
					if author := c.GetAuthor(); author != nil {
						login = author.GetLogin()
						name = author.GetName()
					}
					if name == "" {
						name = login
					}

					authorList := []string{}
					if name != "" {
						if login != "" {
							authorList = append(authorList, fmt.Sprintf("[%s](https://github.com/%s)", name, login))
						} else {
							authorList = append(authorList, name)
						}
					}

					// 添加 Co-authors
					coAuthors := parseCoAuthors(c.GetMessage())
					for _, ca := range coAuthors {
						if ca.Login != "" {
							authorList = append(authorList, fmt.Sprintf("[%s](https://github.com/%s)", ca.Name, ca.Login))
						} else {
							authorList = append(authorList, ca.Name)
						}
					}

					if len(authorList) > 0 {
						authorPart = fmt.Sprintf(" (%s)", strings.Join(authorList, ", "))
					}
				}

				lines = append(lines, fmt.Sprintf("%s %s%s%s", emojiIcon, msg, hashPart, authorPart))
			}
			// 如果某条提交的消息包含 markdown 列表，紧跟 \n 无法断开列表上下文
			// 则在该行后插入空行（\n\n）以强制结束列表
			if len(lines) > 1 {
				var sb strings.Builder
				for i, line := range lines {
					if i > 0 {
						if containsMarkdownList(lines[i-1]) {
							sb.WriteString("\n\n")
						} else {
							sb.WriteString("\n")
						}
					}
					sb.WriteString(line)
				}
				d.Text = sb.String()
			} else {
				d.Text = strings.Join(lines, "\n")
			}
			d.CommitCount = len(e.Commits)
		} else if e.GetDeleted() {
			if isTag {
				d.Title = fmt.Sprintf("🗑️ Tag Deleted: %s", refShort)
				d.IsDeleted = true
			} else {
				d.Title = fmt.Sprintf("🗑️ Branch Deleted: %s", refShort)
				d.IsDeleted = true
			}
			d.Text = fmt.Sprintf("🌿 %s", refShort)
		} else if e.GetCreated() {
			if isTag {
				d.Title = fmt.Sprintf("🏷️ New Tag: %s", refShort)
				d.Text = fmt.Sprintf("🏷️ %s", refShort)
			} else {
				d.Title = fmt.Sprintf("🆕 New Branch: %s", refShort)
				d.Text = ""
			}
		} else if isTag {
			// Tag 推送但没有 commits（可能是已有 tag 的更新）
			d.Title = fmt.Sprintf("🏷️ Tag: %s", refShort)
			d.Text = ""
		}
		if hc := e.GetHeadCommit(); hc != nil {
			d.URL = hc.GetURL()
			sha := hc.GetID()
			d.FullSHA = sha
			if len(sha) > 7 {
				d.SHA = sha[:7]
			} else {
				d.SHA = sha
			}
			if ts := hc.GetTimestamp(); !ts.IsZero() {
				d.EventTime = ts.Time.Format(time.RFC3339)
			}
		}
		d.Action = "push"

	case *github.PullRequestEvent:
		pr := e.GetPullRequest()
		if pr == nil {
			d.Skip = true
			return d
		}
		action := e.GetAction()
		d.Action = action
		switch action {
		case "opened":
			d.Title = "🥕 New PullRequest"
		case "closed":
			if pr.GetMerged() {
				d.Title = "🥕 PullRequest merged"
			} else {
				d.Title = "🥕 PullRequest closed"
			}
		case "reopened":
			d.Title = "🥕 PullRequest reopened"
		case "labeled":
			d.Title = "🏷️ PR Labeled"
		case "unlabeled":
			d.Title = "🏷️ PR Unlabeled"
		default:
			d.Title = fmt.Sprintf("📦 PR %s", action)
		}

		if action == "labeled" || action == "unlabeled" {
			label := ""
			if l := e.GetLabel(); l != nil {
				label = l.GetName()
			}
			d.Text = fmt.Sprintf("**%s**\n\nLabel: `%s`", pr.GetTitle(), label)
		} else {
			text, foldable := ProcessGithubMarkdown(pr.GetBody())
			// 如果内容过长 (比如超过 800 字)，则放入 ExtraReply
			if len(text) > 30000 {
				d.Text = fmt.Sprintf("**%s**\n*(Content too long, see reply)*", pr.GetTitle())
				d.ExtraReply = text
			} else if text != "" {
				d.Text = fmt.Sprintf("**%s**\n%s", pr.GetTitle(), text)
			} else {
				d.Text = fmt.Sprintf("**%s**", pr.GetTitle())
			}
			d.FoldableBody = foldable
		}

		refName := ""
		if head := pr.GetHead(); head != nil {
			refName = head.GetRef()
		}
		if base := pr.GetBase(); base != nil {
			if refName != "" {
				refName = refName + " ➔ " + base.GetRef()
			} else {
				refName = base.GetRef()
			}
		}
		d.RefName = refName
		if head := pr.GetHead(); head != nil {
			if headRepo := head.GetRepo(); headRepo != nil {
				d.RefURL = fmt.Sprintf("%s/tree/%s", headRepo.GetHTMLURL(), head.GetRef())
			}
		}
		d.URL = pr.GetHTMLURL()
		if ts := pr.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.IssuesEvent:
		action := e.GetAction()
		iss := e.GetIssue()
		if iss == nil {
			d.Skip = true
			return d
		}
		switch action {
		case "opened":
			d.Title = "🍄 New Issue"
		case "edited":
			d.Title = "🍄 Issue edited"
		case "closed":
			d.Title = "🍄 Issue closed"
		default:
			d.Title = fmt.Sprintf("🍄 Issue %s", action)
		}
		body, foldable := ProcessGithubMarkdown(iss.GetBody())
		if body != "" {
			d.Text = fmt.Sprintf("**%s**\n%s", iss.GetTitle(), body)
		} else {
			d.Text = fmt.Sprintf("**%s**", iss.GetTitle())
		}
		d.FoldableBody = foldable
		d.URL = iss.GetHTMLURL()
		d.Action = action
		if ts := iss.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.IssueCommentEvent:
		iss := e.GetIssue()
		comment := e.GetComment()
		if iss == nil || comment == nil {
			d.Skip = true
			return d
		}
		action := e.GetAction()
		d.Title = fmt.Sprintf("🌻 Comment %s", action)
		d.Action = action

		body := comment.GetBody()
		if action == "edited" && e.Changes != nil && e.Changes.Body != nil && e.Changes.Body.From != nil {
			body = GetDiffOnlyAdded(*e.Changes.Body.From, body)
		}

		commentBody, foldable := ProcessGithubMarkdown(body)
		d.FoldableBody = foldable
		if len(commentBody) > 10000 {
			d.Text = fmt.Sprintf("**%s**\n*(Comment too long, see reply)*", iss.GetTitle())
			d.ExtraReply = commentBody
		} else if commentBody != "" {
			d.Text = fmt.Sprintf("**%s**\n%s", iss.GetTitle(), commentBody)
		} else {
			d.Text = fmt.Sprintf("**%s**", iss.GetTitle())
		}
		d.URL = e.GetComment().GetHTMLURL()
		if ts := comment.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.PullRequestReviewCommentEvent:
		pr := e.GetPullRequest()
		comment := e.GetComment()
		if pr == nil || comment == nil {
			d.Skip = true
			return d
		}
		action := e.GetAction()
		d.Title = fmt.Sprintf("💬 PR Comment %s", action)
		d.Action = action

		body := comment.GetBody()
		if action == "edited" && e.Changes != nil && e.Changes.Body != nil && e.Changes.Body.From != nil {
			body = GetDiffOnlyAdded(*e.Changes.Body.From, body)
		}

		commentBody, foldable := ProcessGithubMarkdown(body)
		d.FoldableBody = foldable
		if commentBody != "" {
			d.Text = fmt.Sprintf("**%s**\n%s", pr.GetTitle(), commentBody)
		} else {
			d.Text = fmt.Sprintf("**%s**", pr.GetTitle())
		}
		d.URL = comment.GetHTMLURL()
		if ts := comment.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.PullRequestReviewEvent:
		pr := e.GetPullRequest()
		review := e.GetReview()
		if pr == nil || review == nil {
			d.Skip = true
			return d
		}
		action := e.GetAction()
		d.Title = fmt.Sprintf("🧐 PR Review %s", action)
		d.Action = action

		body := review.GetBody()
		// PullRequestReviewEvent 在 go-github 中目前没有 Changes 字段
		reviewBody, foldable := ProcessGithubMarkdown(body)
		d.FoldableBody = foldable
		if reviewBody != "" {
			d.Text = fmt.Sprintf("**%s**\n%s", pr.GetTitle(), reviewBody)
		} else {
			d.Text = fmt.Sprintf("**%s**", pr.GetTitle())
		}
		d.URL = review.GetHTMLURL()
		if ts := review.GetSubmittedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.WorkflowRunEvent:
		wr := e.GetWorkflowRun()
		if wr == nil {
			d.Skip = true
			return d
		}
		status := wr.GetStatus()
		conclusion := wr.GetConclusion()
		workflowName := wr.GetName()
		ref := wr.GetHeadBranch()
		sha := wr.GetHeadSHA()
		shortSHA := sha
		if len(sha) > 7 {
			shortSHA = sha[:7]
		}

		icon := "⚙️"
		stateVerb := "started"
		switch conclusion {
		case "success":
			icon = "✅"
			stateVerb = "succeeded"
		case "failure", "cancelled", "timed_out":
			icon = "❌"
			if conclusion == "failure" {
				stateVerb = "failed"
			} else {
				stateVerb = conclusion
			}
		default:
			if status == "in_progress" {
				icon = "⏳"
				stateVerb = "running"
			}
		}

		d.SHA = shortSHA
		repoUrl := ""
		if repo := e.GetRepo(); repo != nil {
			repoUrl = repo.GetHTMLURL()
		}
		if repoUrl != "" && ref != "" {
			d.RefURL = fmt.Sprintf("%s/tree/%s", repoUrl, ref)
		}
		d.Title = fmt.Sprintf("%s Workflow %s: %s", icon, titleCase(stateVerb), workflowName)

		var lines []string
		durationPart := ""
		if conclusion != "" {
			startedAt := wr.GetRunStartedAt()
			updatedAt := wr.GetUpdatedAt()
			if !startedAt.IsZero() && !updatedAt.IsZero() {
				start := startedAt.Time
				end := updatedAt.Time
				if !start.IsZero() && !end.IsZero() {
					durationPart = fmt.Sprintf(" in %s", FormatDuration(end.Sub(start)))
				}
			}
		}
		lines = append(lines, fmt.Sprintf("%s **%s** workflow run %s%s", icon, workflowName, stateVerb, durationPart))

		d.Text = strings.Join(lines, "\n")
		d.RefName = ref
		d.URL = wr.GetHTMLURL()
		if ts := wr.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.WorkflowJobEvent:
		wj := e.GetWorkflowJob()
		if wj == nil {
			d.Skip = true
			return d
		}
		status := wj.GetStatus()
		conclusion := wj.GetConclusion()
		jobName := wj.GetName()
		shortSHA := wj.GetHeadSHA()
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		d.SHA = shortSHA

		icon := "⚙️"
		stateVerb := "started"
		switch conclusion {
		case "success":
			icon = "✅"
			stateVerb = "succeeded"
		case "failure", "cancelled", "timed_out":
			icon = "❌"
			stateVerb = conclusion
		default:
			if status == "in_progress" {
				icon = "⏳"
				stateVerb = "running"
			}
		}

		d.Title = fmt.Sprintf("%s Job %s: %s", icon, titleCase(stateVerb), jobName)

		var lines []string
		// 如果有 workflow_name 则显示为 Workflow / Job 格式
		displayJobName := jobName
		if wj.GetWorkflowName() != "" {
			displayJobName = fmt.Sprintf("%s / %s", wj.GetWorkflowName(), jobName)
		}

		durationPart := ""
		if conclusion != "" {
			startedAt := wj.GetStartedAt()
			completedAt := wj.GetCompletedAt()
			if !startedAt.IsZero() && !completedAt.IsZero() {
				start := startedAt.Time
				end := completedAt.Time
				if !start.IsZero() && !end.IsZero() {
					durationPart = fmt.Sprintf(" in %s", FormatDuration(end.Sub(start)))
				}
			}
		}
		lines = append(lines, fmt.Sprintf("%s job **%s** %s%s", icon, displayJobName, stateVerb, durationPart))

		d.Text = strings.Join(lines, "\n")
		d.RefName = wj.GetHeadBranch()
		if repo := e.GetRepo(); repo != nil {
			repoUrl := repo.GetHTMLURL()
			if repoUrl != "" && wj.GetHeadBranch() != "" {
				d.RefURL = fmt.Sprintf("%s/tree/%s", repoUrl, wj.GetHeadBranch())
			}
		}
		d.URL = wj.GetHTMLURL()
		if ts := wj.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.CheckRunEvent:
		cr := e.GetCheckRun()
		if cr == nil {
			d.Skip = true
			return d
		}
		status := cr.GetStatus()
		conclusion := cr.GetConclusion()
		name := cr.GetName()
		sha := cr.GetHeadSHA()
		shortSHA := sha
		if len(sha) > 7 {
			shortSHA = sha[:7]
		}
		d.SHA = shortSHA

		icon := "⚙️"
		stateVerb := "started"
		switch conclusion {
		case "success":
			icon = "✅"
			stateVerb = "succeeded"
		case "failure", "cancelled", "timed_out":
			icon = "❌"
			stateVerb = conclusion
		default:
			if status == "in_progress" {
				icon = "⏳"
				stateVerb = "running"
			}
		}

		d.Title = fmt.Sprintf("%s Check: %s", icon, name)
		d.Text = fmt.Sprintf("%s check **%s** %s", icon, name, stateVerb)
		d.URL = cr.GetHTMLURL()
		d.RefName = e.GetCheckRun().GetCheckSuite().GetHeadBranch()

	case *github.CheckSuiteEvent:
		cs := e.GetCheckSuite()
		if cs == nil {
			d.Skip = true
			return d
		}
		status := cs.GetStatus()
		conclusion := cs.GetConclusion()
		sha := cs.GetHeadSHA()
		shortSHA := sha
		if len(sha) > 7 {
			shortSHA = sha[:7]
		}
		d.SHA = shortSHA

		icon := "⚙️"
		stateVerb := "started"
		switch conclusion {
		case "success":
			icon = "✅"
			stateVerb = "succeeded"
		case "failure", "cancelled", "timed_out":
			icon = "❌"
			stateVerb = conclusion
		default:
			if status == "in_progress" {
				icon = "⏳"
				stateVerb = "running"
			}
		}

		d.Title = fmt.Sprintf("%s Check Suite %s", icon, titleCase(stateVerb))
		d.Text = fmt.Sprintf("%s check suite %s", icon, stateVerb)
		d.URL = "" // CheckSuite does not have a direct HTMLURL field in go-github
		d.RefName = cs.GetHeadBranch()
		if ts := cs.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.WatchEvent:
		d.Title = "⭐ New Star!"
		d.Text = "Your repository has a new follower."

	case *github.StarEvent:
		action := e.GetAction()
		if action == "deleted" {
			d.Title = "💔 Star Removed"
		} else {
			d.Title = "⭐ New Star!"
		}
		d.Text = ""

	case *github.ForkEvent:
		forkee := e.GetForkee()
		if forkee == nil {
			d.Skip = true
			return d
		}
		d.Title = "🍴 Repository Forked"
		d.Text = fmt.Sprintf("Repository forked to [%s](%s)", forkee.GetFullName(), forkee.GetHTMLURL())
		if ts := forkee.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.GollumEvent:
		d.Title = "📖 Wiki Updated"
		var pages []string
		for _, p := range e.Pages {
			pages = append(pages, fmt.Sprintf("• [%s](%s) (%s)", p.GetTitle(), p.GetHTMLURL(), p.GetAction()))
		}
		d.Text = strings.Join(pages, "\n")

	case *github.CreateEvent:
		if e.GetRefType() == "tag" {
			ref := e.GetRef()
			repoUrl := ""
			if repo := e.GetRepo(); repo != nil {
				repoUrl = repo.GetHTMLURL()
			}
			d.Title = fmt.Sprintf("🏷️ New Tag: %s", ref)
			d.RefName = ref
			if repoUrl != "" {
				d.RefURL = fmt.Sprintf("%s/releases/tag/%s", repoUrl, ref)
				d.URL = d.RefURL
			}
			d.IsTag = true
			d.Text = fmt.Sprintf("🏷️ %s", ref)
		} else {
			// 分支创建通常由 Push 事件处理，这里跳过
			d.Skip = true
		}

	case *github.DeleteEvent:
		ref := e.GetRef()
		if e.GetRefType() == "tag" {
			d.Title = fmt.Sprintf("🗑️ Tag Deleted: %s", ref)
			d.RefName = ref
			d.IsTag = true
			d.IsDeleted = true
			d.Text = fmt.Sprintf("🗑️ %s", ref)
		} else {
			d.Title = fmt.Sprintf("🗑️ Branch Deleted: %s", ref)
			d.RefName = ref
			d.IsDeleted = true
			d.Text = fmt.Sprintf("🗑️ %s", ref)
		}

	case *github.PublicEvent:
		d.Title = "🔓 Repository Made Public"
		d.Text = "This repository is now visible to everyone."

	case *github.RepositoryEvent:
		if repo := e.GetRepo(); repo != nil {
			if ts := repo.GetCreatedAt(); !ts.IsZero() {
				d.EventTime = ts.Format(time.RFC3339)
			}
		}
		action := e.GetAction()
		switch action {
		case "publicized":
			// GitHub 同时会发送 public 事件，这里直接跳过以防重复
			d.Skip = true
		case "privatized":
			d.Title = "🔒 Repository Made Private"
		case "deleted":
			d.Title = "🗑️ Repository Deleted"
		case "renamed":
			d.Title = "📝 Repository Renamed"
			if repo := e.GetRepo(); repo != nil {
				d.Text = fmt.Sprintf("Renamed to **%s**", repo.GetFullName())
			}
		default:
			// 其他 edited 事件（如修改描述、Logo 等）通常比较琐碎，默认跳过
			d.Skip = true
		}
		d.Action = action

	case *github.OrganizationEvent:
		org := e.GetOrganization()
		membership := e.GetMembership()
		if org == nil || membership == nil {
			d.Skip = true
			return d
		}
		member := membership.GetUser()
		if member == nil {
			d.Skip = true
			return d
		}
		login := member.GetLogin()
		if login == "****" || login == "" {
			// 如果是邀请，尝试从其他地方获取信息（如暂时显示为 "New Member"）
			if login == "" {
				login = "Someone"
			}
		}
		d.Title = fmt.Sprintf("🏢 Org %s: %s", org.GetLogin(), e.GetAction())
		text := fmt.Sprintf("Action: **%s**\nMember: **%s**", e.GetAction(), login)
		if sender := e.GetSender(); sender != nil && sender.GetLogin() != login {
			text += fmt.Sprintf("\nBy: **%s**", sender.GetLogin())
		}
		d.Text = text
		d.Action = e.GetAction()
		d.URL = org.GetHTMLURL()
		if login != "" && login != "****" {
			d.AuthorLogins = []string{login}
			d.AuthorAvatars = []string{member.GetAvatarURL()}
		}
		if sender := e.GetSender(); sender != nil && sender.GetLogin() != login {
			d.AuthorLogins = append(d.AuthorLogins, sender.GetLogin())
			d.AuthorAvatars = append(d.AuthorAvatars, sender.GetAvatarURL())
		}
		if ts := org.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.TeamEvent:
		team := e.GetTeam()
		if team == nil {
			d.Skip = true
			return d
		}
		d.Title = fmt.Sprintf("👥 Team %s: %s", team.GetName(), e.GetAction())
		d.Text = fmt.Sprintf("Action: **%s**\nTeam: **%s**", e.GetAction(), team.GetName())
		if repo := e.GetRepo(); repo != nil {
			d.Text += fmt.Sprintf("\nRepo: **%s**", repo.GetFullName())
		}
		d.Action = e.GetAction()
		d.URL = team.GetHTMLURL()

	case *github.MemberEvent:
		member := e.GetMember()
		if member == nil {
			d.Skip = true
			return d
		}
		d.Title = fmt.Sprintf("👤 Member %s: %s", member.GetLogin(), e.GetAction())
		text := fmt.Sprintf("Action: **%s**\nMember: **%s**", e.GetAction(), member.GetLogin())
		if sender := e.GetSender(); sender != nil && sender.GetLogin() != member.GetLogin() {
			text += fmt.Sprintf("\nBy: **%s**", sender.GetLogin())
		}
		d.Text = text
		d.Action = e.GetAction()
		d.URL = member.GetHTMLURL()
		d.AuthorLogins = []string{member.GetLogin()}
		d.AuthorAvatars = []string{member.GetAvatarURL()}
		if sender := e.GetSender(); sender != nil && sender.GetLogin() != member.GetLogin() {
			d.AuthorLogins = append(d.AuthorLogins, sender.GetLogin())
			d.AuthorAvatars = append(d.AuthorAvatars, sender.GetAvatarURL())
		}
		if ts := member.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.ReleaseEvent:
		action := e.GetAction()
		release := e.GetRelease()
		repo := e.GetRepo()
		if release == nil {
			d.Skip = true
			return d
		}
		d.Action = action
		d.URL = release.GetHTMLURL()

		switch action {
		case "published":
			d.Title = fmt.Sprintf("🚀 Release Published: %s", release.GetName())
		case "unpublished":
			d.Title = fmt.Sprintf("🚫 Release Unpublished: %s", release.GetName())
		case "created":
			d.Title = fmt.Sprintf("📝 Release Created: %s", release.GetName())
		case "edited":
			d.Title = fmt.Sprintf("✏️ Release Edited: %s", release.GetName())
		case "deleted":
			d.Title = fmt.Sprintf("🗑️ Release Deleted: %s", release.GetName())
		case "prereleased":
			d.Title = fmt.Sprintf("🧪 Pre-release: %s", release.GetName())
		case "released":
			d.Title = fmt.Sprintf("🏆 Release: %s", release.GetName())
		default:
			d.Title = fmt.Sprintf("📦 Release %s: %s", action, release.GetName())
		}

		// 构建发布内容
		var lines []string
		if tag := release.GetTagName(); tag != "" {
			d.RefName = tag
			if repo != nil {
				d.RefURL = fmt.Sprintf("%s/releases/tag/%s", repo.GetHTMLURL(), tag)
			}
			lines = append(lines, fmt.Sprintf("**Tag:** `%s`", tag))
		}
		if author := release.GetAuthor(); author != nil {
			lines = append(lines, fmt.Sprintf("**Author:** [%s](https://github.com/%s)", author.GetLogin(), author.GetLogin()))
			d.AuthorLogins = []string{author.GetLogin()}
			d.AuthorAvatars = []string{author.GetAvatarURL()}
		}
		if body := release.GetBody(); body != "" {
			text, _ := ProcessGithubMarkdown(body)
			if text != "" {
				lines = append(lines, "", "**Release Notes:**", text)
			}
		}
		d.Text = strings.Join(lines, "\n")
		if ts := release.GetCreatedAt(); !ts.IsZero() {
			d.EventTime = ts.Format(time.RFC3339)
		}

	case *github.MembershipEvent:
		member := e.GetMember()
		if member == nil {
			d.Skip = true
			return d
		}
		d.Title = fmt.Sprintf("👥 Membership %s: %s", member.GetLogin(), e.GetAction())
		text := fmt.Sprintf("Action: **%s**\nMember: **%s**\nScope: **%s**", e.GetAction(), member.GetLogin(), e.GetScope())
		if sender := e.GetSender(); sender != nil && sender.GetLogin() != member.GetLogin() {
			text += fmt.Sprintf("\nBy: **%s**", sender.GetLogin())
		}
		d.Text = text
		d.Action = e.GetAction()
		d.AuthorLogins = []string{member.GetLogin()}
		d.AuthorAvatars = []string{member.GetAvatarURL()}
		if sender := e.GetSender(); sender != nil && sender.GetLogin() != member.GetLogin() {
			d.AuthorLogins = append(d.AuthorLogins, sender.GetLogin())
			d.AuthorAvatars = append(d.AuthorAvatars, sender.GetAvatarURL())
		}
	}
	return d
}

// splitCommits 将 push 事件的文本按 commit 条目拆分
// 每个 commit 以 🔸 或 🔹 开头，占一行（可能包含多行子内容）
func splitCommits(text string) []string {
	lines := strings.Split(text, "\n")
	var commits []string
	var current strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "🔸 ") || strings.HasPrefix(line, "🔹 ") {
			if current.Len() > 0 {
				commits = append(commits, current.String())
				current.Reset()
			}
			current.WriteString(line)
		} else if current.Len() > 0 {
			current.WriteString("\n")
			current.WriteString(line)
		}
	}
	if current.Len() > 0 {
		commits = append(commits, current.String())
	}
	return commits
}

// titleCase 将字符串首字母大写（替代已废弃的 strings.Title）
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] -= 32
	}
	return string(runes)
}

// cardColor 枚举卡片标题颜色，避免依赖标题 emoji 做字符串匹配
type cardColor string

const (
	cardColorBlue   cardColor = "blue"
	cardColorGreen  cardColor = "green"
	cardColorRed    cardColor = "red"
	cardColorOrange cardColor = "orange"
	cardColorGrey   cardColor = "grey"
	cardColorPurple cardColor = "purple"
)

// GetTemplate 根据标题中的 emoji 或关键字返回对应的飞书卡片标题色
// 支持颜色: blue / green / red / orange / grey / purple / indigo / wathet / turquoise / yellow / lime / pink / carmine
func GetTemplate(title string) string {
	if ContainsAny(title, "❌", "💥", "💔", "failed", "Failure", "Failed") {
		return string(cardColorRed)
	}
	if ContainsAny(title, "✅", "succeeded", "Success", "Succeeded") {
		return string(cardColorGreen)
	}
	if ContainsAny(title, "⏳", "🏃", "running", "Started", "Running") {
		return string(cardColorOrange)
	}
	if ContainsAny(title, "🗑️", "Deleted") {
		return string(cardColorGrey)
	}
	if ContainsAny(title, "🏷️", "Tag", "New Tag") {
		return string(cardColorPurple)
	}
	if ContainsAny(title, "🚀", "Release", "Pre-release") {
		return "turquoise"
	}
	if ContainsAny(title, "🆕", "New Branch", "New Commits", "commits") {
		return "wathet"
	}
	if ContainsAny(title, "🥕", "PullRequest", "PR") {
		return "indigo"
	}
	return string(cardColorBlue)
}

// extractPRNumber 从 commit message 中提取 PR 编号（匹配 "Merge pull request #N" 模式）
func extractPRNumber(msg string) string {
	matches := prMergeRegex.FindStringSubmatch(msg)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseRunIDFromURL 从 GitHub Actions URL 中解析 run ID
// URL 格式: https://github.com/owner/repo/actions/runs/123456
func parseRunIDFromURL(url string) int64 {
	if url == "" {
		return 0
	}
	// 查找 /runs/ 后的数字
	idx := strings.LastIndex(url, "/runs/")
	if idx < 0 {
		return 0
	}
	remaining := url[idx+6:]
	// 提取连续数字
	var numStr string
	for _, c := range remaining {
		if c >= '0' && c <= '9' {
			numStr += string(c)
		} else {
			break
		}
	}
	if numStr == "" {
		return 0
	}
	var id int64
	fmt.Sscanf(numStr, "%d", &id)
	return id
}

// getCIStatusesForParent 查询关联到指定父消息的所有 CI 事件状态
// 从每条 CI 记录的 EventDetail 构造 CIStatus（CI 记录不单独存储 CIStatuses）
func getCIStatusesForParent(ctx context.Context, parentMsgID string) []CIStatus {
	if DB == nil || parentMsgID == "" {
		return nil
	}
	var records []MessageRecord
	if err := DB.NewSelect().Model(&records).
		Where("parent_msg_id = ?", parentMsgID).
		Where("event_type IN ('workflow_run', 'workflow_job', 'check_run', 'check_suite')").
		Order("id DESC"). // 最新的优先，去重时保留最新状态
		Scan(ctx); err != nil {
		return nil
	}
	var statuses []CIStatus
	seen := make(map[string]bool)
	for _, r := range records {
		var detail EventDetail
		_ = json.Unmarshal([]byte(r.Content), &detail)
		// 从 EventDetail 的 Title 提取 CI 状态信息
		// Title 格式: "✅ Workflow Succeeded: CI" / "❌ Workflow Failed: CI" / "✅ Check: lint"
		workflowName := ""
		titleParts := strings.SplitN(detail.Title, ": ", 2)
		if len(titleParts) > 1 {
			workflowName = titleParts[1]
		} else {
			workflowName = detail.Title
		}
		conclusion := ""
		status := "completed"
		if strings.Contains(detail.Title, "✅") {
			conclusion = "success"
		} else if strings.Contains(detail.Title, "❌") {
			conclusion = "failure"
		} else if strings.Contains(detail.Title, "⏳") {
			status = "in_progress"
		} else if strings.Contains(detail.Title, "⚙️") {
			status = "in_progress"
		}
		key := workflowName
		if !seen[key] {
			seen[key] = true
			// 从 Text 中提取耗时信息
			duration := ""
			if idx := strings.Index(detail.Text, " in "); idx >= 0 {
				duration = strings.TrimSpace(detail.Text[idx+4:])
			}
			statuses = append(statuses, CIStatus{
				WorkflowName: workflowName,
				Status:       status,
				Conclusion:   conclusion,
				RunID:        parseRunIDFromURL(detail.URL),
				Duration:     duration,
				UpdatedAt:    r.UpdatedAt.Format(time.RFC3339),
			})
		}
	}
	return statuses
}

// renderCIStatuses 将 CI 状态列表渲染为飞书 markdown
func renderCIStatuses(statuses []CIStatus, repoURL string) string {
	if len(statuses) == 0 {
		return ""
	}

	// 分离 workflow 和 job 条目
	type workflowGroup struct {
		workflow CIStatus
		jobs     []CIStatus
	}
	var groups []workflowGroup
	workflowMap := make(map[string]*workflowGroup)

	for _, cs := range statuses {
		if strings.HasPrefix(cs.WorkflowName, "job:") {
			// job 条目：提取 parent_run_id 关联到 workflow
			if cs.ParentRunID > 0 {
				key := fmt.Sprintf("%d", cs.ParentRunID)
				if g, ok := workflowMap[key]; ok {
					g.jobs = append(g.jobs, cs)
				}
			}
		} else {
			// workflow 条目
			key := fmt.Sprintf("%d", cs.RunID)
			g := workflowGroup{workflow: cs}
			workflowMap[key] = &g
			groups = append(groups, g)
		}
	}
	// 更新 groups 中的 jobs
	for i := range groups {
		key := fmt.Sprintf("%d", groups[i].workflow.RunID)
		if g, ok := workflowMap[key]; ok {
			groups[i].jobs = g.jobs
		}
	}

	var lines []string
	for _, g := range groups {
		lines = append(lines, renderSingleCIStatus(g.workflow, repoURL, false))
		for _, job := range g.jobs {
			lines = append(lines, renderSingleCIStatus(job, repoURL, true))
		}
	}
	return strings.Join(lines, "\n")
}

// renderSingleCIStatus 渲染单条 CI 状态
func renderSingleCIStatus(cs CIStatus, repoURL string, isJob bool) string {
	icon := "⏳"
	statusText := cs.Status
	switch cs.Conclusion {
	case "success":
		icon = "✅"
		statusText = "passed"
	case "failure":
		icon = "❌"
		statusText = "failed"
	case "cancelled":
		icon = "🚫"
		statusText = "cancelled"
	default:
		if cs.Status == "in_progress" {
			statusText = "running"
		} else if cs.Status == "queued" || cs.Status == "waiting" {
			icon = "⏳"
			statusText = "pending"
		}
	}
	durationPart := ""
	if cs.Duration != "" {
		durationPart = " (" + cs.Duration + ")"
	}
	runLink := ""
	if repoURL != "" && cs.RunID > 0 && !isJob {
		runLink = fmt.Sprintf(" ([logs](%s/actions/runs/%d))", repoURL, cs.RunID)
	}

	// 提取显示名称
	displayName := cs.WorkflowName
	if strings.HasPrefix(displayName, "job:") {
		displayName = cs.JobName // 使用存储的 job 名称
	}

	if isJob {
		return fmt.Sprintf("    %s %s **%s**%s", icon, displayName, statusText, durationPart)
	}
	return fmt.Sprintf("%s %s **%s**%s%s", icon, displayName, statusText, durationPart, runLink)
}

// ciFailed 检查 CI 状态列表中是否有失败的
func ciFailed(statuses []CIStatus) bool {
	for _, cs := range statuses {
		if cs.Conclusion == "failure" || cs.Conclusion == "cancelled" {
			return true
		}
	}
	return false
}

// makeCIActionButtons 为失败的 CI 事件生成操作按钮
func makeCIActionButtons(statuses []CIStatus, repoURL string) []ActionButton {
	if repoURL == "" {
		return nil
	}
	var btns []ActionButton
	seenRuns := make(map[int64]bool)
	for _, cs := range statuses {
		if (cs.Conclusion == "failure" || cs.Conclusion == "cancelled") && cs.RunID > 0 && !seenRuns[cs.RunID] {
			seenRuns[cs.RunID] = true
			btns = append(btns, ActionButton{
				Text: fmt.Sprintf("View %s Logs", cs.WorkflowName),
				URL:  fmt.Sprintf("%s/actions/runs/%d", repoURL, cs.RunID),
				Type: "danger",
			})
		}
	}
	if len(btns) > 1 {
		btns = btns[:1]
	}
	if len(btns) > 0 {
		btns = append(btns, ActionButton{
			Text: "View All Workflows",
			URL:  repoURL + "/actions",
			Type: "default",
		})
	}
	return btns
}

// BuildCard 构建符合飞书卡片 V2 规范的消息卡片
func BuildCard(ctx context.Context, repo, sender, senderUrl, avatarUrl string, detail EventDetail) *Card {
	card := NewCard()
	card.Header.Title = CardText{Tag: "plain_text", Content: detail.Title}
	card.Header.Template = GetTemplate(detail.Title)
	repoUrl := detail.RepoURL
	if repoUrl == "" && repo != "" {
		repoUrl = fmt.Sprintf("https://github.com/%s", repo)
	}

	// 删除事件：标题改为仓库名，避免冗余
	if detail.IsDeleted && repo != "" {
		card.Header.Title = CardText{Tag: "plain_text", Content: fmt.Sprintf("%s: %s", strings.SplitN(detail.Title, ":", 2)[0], repo)}
	}

	// --- 1. 摘要信息行：仓库 / 分支 / 提交人（含头像） ---
	repoPart := ""
	if repo != "" {
		repoPart = fmt.Sprintf("📦 [%s](%s)", repo, repoUrl)
	}

	refPart := ""
	if detail.RefName != "" {
		link := detail.RefURL
		if link == "" {
			link = repoUrl
		}
		shaPart := ""
		if detail.SHA != "" && repoUrl != "" {
			sha := detail.FullSHA
			if sha == "" {
				sha = detail.SHA
			}
			shaPart = fmt.Sprintf(" ([`%s`](%s/commit/%s))", detail.SHA, repoUrl, sha)
		}
		if detail.IsTag {
			refPart = fmt.Sprintf("🏷️ [%s](%s)%s", detail.RefName, link, shaPart)
		} else {
			refPart = fmt.Sprintf("🌿 [%s](%s)%s", detail.RefName, link, shaPart)
		}
	}

	var metaParts []string
	if repoPart != "" {
		metaParts = append(metaParts, repoPart)
	}
	if refPart != "" {
		metaParts = append(metaParts, refPart)
	}
	metaText := strings.Join(metaParts, " / ")

	// 构建发送者文本
	senderText := fmt.Sprintf("[%s](%s)", sender, senderUrl)
	if len(detail.AuthorLogins) > 1 {
		var links []string
		for _, login := range detail.AuthorLogins {
			links = append(links, fmt.Sprintf("[%s](https://github.com/%s)", login, login))
		}
		senderText = strings.Join(links, "  ")
	} else if len(detail.AuthorLogins) == 1 {
		login := detail.AuthorLogins[0]
		senderText = fmt.Sprintf("[%s](https://github.com/%s)", login, login)
	}

	// 收集最多 3 个头像的 img_key（飞书列数有上限，超出会导致排版混乱）
	avatarsToDisplay := detail.AuthorAvatars
	if len(avatarsToDisplay) == 0 && avatarUrl != "" {
		avatarsToDisplay = []string{avatarUrl}
	}
	const maxAvatars = 3
	if len(avatarsToDisplay) > maxAvatars {
		avatarsToDisplay = avatarsToDisplay[:maxAvatars]
	}

	var resolvedAvatars []string // 已缓存的 img_key 列表
	for _, u := range avatarsToDisplay {
		if key := GetImageKey(ctx, u); key != "" {
			resolvedAvatars = append(resolvedAvatars, key)
		}
	}

	// 删除事件：简洁单行 body，不显示冗余仓库/分支摘要
	if detail.IsDeleted {
		tagIcon := "🗑️🌿"
		if detail.IsTag {
			tagIcon = "🗑️🏷️"
		}
		refLink := ""
		if detail.RefName != "" && detail.RefURL != "" {
			refLink = fmt.Sprintf(" [%s](%s)", detail.RefName, detail.RefURL)
		} else if detail.RefName != "" {
			refLink = fmt.Sprintf(" `%s`", detail.RefName)
		}
		senderText := fmt.Sprintf("[%s](%s)", sender, senderUrl)
		if len(detail.AuthorLogins) == 1 {
			login := detail.AuthorLogins[0]
			senderText = fmt.Sprintf("[%s](https://github.com/%s)", login, login)
		}
		content := fmt.Sprintf("%s%s 👤 %s", tagIcon, refLink, senderText)
		if len(resolvedAvatars) > 0 {
			avatarEls := make([]any, 0, len(resolvedAvatars))
			for _, key := range resolvedAvatars {
				avatarEls = append(avatarEls, map[string]any{
					"tag":          "img",
					"img_key":      key,
					"custom_width": 20,
					"mode":         "crop_center",
					"alt":          map[string]string{"tag": "plain_text", "content": "avatar"},
				})
			}
			card.Body.Elements = append(card.Body.Elements, map[string]any{
				"tag":                "column_set",
				"flex_mode":          "none",
				"horizontal_spacing": "small",
				"columns": []any{
					map[string]any{
						"tag": "column", "width": "weighted", "weight": 1,
						"vertical_align": "center",
						"elements":       []any{map[string]any{"tag": "markdown", "content": content}},
					},
					map[string]any{
						"tag": "column", "width": "auto",
						"vertical_align": "center",
						"elements":       avatarEls,
					},
				},
			})
		} else {
			card.AddMarkdown(content)
		}
		// 删除事件也加 View Details 按钮
		btnURL := repoUrl
		if detail.URL != "" {
			btnURL = detail.URL
		}
		if btnURL != "" {
			card.AddActions("flow", ActionButton{Text: "View Details", URL: btnURL, Type: "default"})
		}
	} else {

	// 构建摘要行：用 column_set 排列 [meta文本] [头像...] [发送者]
	// 头像全部合并进一个列（inline 排列），避免列数过多
	if len(resolvedAvatars) > 0 {
		// 将所有头像以小图标方式拼成一段 markdown（飞书 lark_md 不支持 img，
		// 所以头像列仍用独立 img 元素，但合并到单个 column 的 elements 数组里）
		avatarEls := make([]any, 0, len(resolvedAvatars))
		for _, key := range resolvedAvatars {
			avatarEls = append(avatarEls, map[string]any{
				"tag":          "img",
				"img_key":      key,
				"custom_width": 20,
				"mode":         "crop_center",
				"alt": map[string]string{
					"tag":     "plain_text",
					"content": "avatar",
				},
			})
		}

		columns := []any{
			// 左列：仓库+分支
			map[string]any{
				"tag":            "column",
				"width":          "weighted",
				"weight":         3,
				"vertical_align": "center",
				"elements": []any{
					map[string]any{"tag": "markdown", "content": metaText},
				},
			},
			// 中列：头像（多个 img 叠在同一列）
			map[string]any{
				"tag":            "column",
				"width":          "auto",
				"vertical_align": "center",
				"elements":       avatarEls,
			},
			// 右列：发送者链接
			map[string]any{
				"tag":            "column",
				"width":          "weighted",
				"weight":         2,
				"vertical_align": "center",
				"elements": []any{
					map[string]any{"tag": "markdown", "content": senderText},
				},
			},
		}

		card.Body.Elements = append(card.Body.Elements, map[string]any{
			"tag":                "column_set",
			"flex_mode":          "none",
			"horizontal_spacing": "small",
			"columns":            columns,
		})
	} else {
		// 无头像缓存时退回到纯文本摘要行
		line := "👤 " + senderText
		if metaText != "" {
			line = metaText + " / " + line
		}
		card.AddMarkdown(line)
	}

	// --- 2. 详情内容 ---
	if detail.Text != "" {
		card.AddDivider()
		// Push 事件：超过 3 条 commit 时按 commit 折叠
		commitCount := detail.CommitCount
		if commitCount == 0 {
			commitCount = detail.EventCount
		}
		if detail.Action == "push" && !detail.IsDeleted && commitCount > 3 {
			commits := splitCommits(detail.Text)
			visible := strings.Join(commits[:3], "\n")
			remaining := strings.Join(commits[3:], "\n")
			card.AddMarkdown(visible)
			card.AddCollapsiblePanel(fmt.Sprintf("📝 展开查看其余 %d 条提交", len(commits)-3), remaining)
		} else {
			card.AddMarkdown(detail.Text)
		}
	}

	// --- 2.5 CI 状态（内联到触发事件的卡片）---
	if ciText := renderCIStatuses(detail.CIStatuses, repoUrl); ciText != "" {
		card.AddDivider()
		card.AddMarkdown(ciText)
	}

	// --- 3. 可折叠的附加内容（PR body 中的 <details> 块等）---
	if detail.FoldableBody != "" {
		card.AddCollapsiblePanel("📝 展开查看详情", detail.FoldableBody)
	}

	// --- 4. 操作按钮（V2 规范：必须放在 action 容器内）---
	// CI 失败时优先显示 CI 操作按钮
	var btns []ActionButton
	if ciFailed(detail.CIStatuses) {
		btns = makeCIActionButtons(detail.CIStatuses, repoUrl)
	}
	// Push / 删除 / 新建分支等事件不显示详情按钮
	skipBtn := strings.Contains(detail.Title, "commits") ||
		strings.Contains(detail.Title, "Deleted") ||
		strings.Contains(detail.Title, "Created")
	if detail.URL != "" && !skipBtn {
		btnType := "primary"
		if ciFailed(detail.CIStatuses) || ContainsAny(detail.Title, "❌", "💥") {
			btnType = "danger"
		}
		btns = append(btns, ActionButton{Text: "View Details", URL: detail.URL, Type: btnType})
	}
	if len(btns) > 0 {
		card.AddActions("flow", btns...)
	}
	} // end of non-delete rendering

	// --- 5. 事件发生时间 ---
	if detail.EventTime != "" {
		if t, err := time.Parse(time.RFC3339, detail.EventTime); err == nil {
			loc, _ := time.LoadLocation("Asia/Shanghai")
			timeStr := t.In(loc).Format("2006-01-02 15:04:05")
			// 合并事件显示时间范围
			if detail.EventTimeEnd != "" {
				if t2, err2 := time.Parse(time.RFC3339, detail.EventTimeEnd); err2 == nil {
					timeStr += " ~ " + t2.In(loc).Format("15:04:05")
				}
			}
			card.AddMarkdown(fmt.Sprintf("🕐 %s", timeStr))
		}
	}

	return card
}

// ContainsAny 检查字符串是否包含任意一个子串
func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// SafeText 安全地截断字符串到指定长度，避免 UTF8 字节切分问题，
// 并将 < 和 > 替换为全角字符，防止飞书内部 Markdown 解析错误。
func SafeText(s string, maxRunes int) string {
	if s == "" {
		return ""
	}

	s = strings.ReplaceAll(s, "<", "＜")
	s = strings.ReplaceAll(s, ">", "＞")

	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return s
}

// truncateAtLine 在行边界处截断文本，保留不超过 maxRunes 个字符的完整行
func truncateAtLine(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	truncated := string(runes[:maxRunes])
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated + "\n..."
}

var conventionalRegex = regexp.MustCompile(`(?i)(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|ref)(\([^)]+\))?(!?):`)
var shaRegex = regexp.MustCompile(`\b([0-9a-f]{7,40})\b`)
var issueRegex = regexp.MustCompile(`(?i)(?:^|[\s,.\-=(])#(\d+)\b`)
var prMergeRegex = regexp.MustCompile(`(?i)Merge pull request #(\d+)`)

// Linkify 将文本中的 SHA 哈希和 #123 转换为 GitHub 链接
func Linkify(text, repoUrl string) string {
	if repoUrl == "" {
		return text
	}

	// 1. 处理 SHA 哈希 (7-40位 16 进制)
	text = shaRegex.ReplaceAllStringFunc(text, func(sha string) string {
		// 启发式校验 SHA: 如果主要是数字且很短，则可能是版本号或其他 ID
		hasLetter := false
		for _, r := range sha {
			if (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				hasLetter = true
				break
			}
		}
		// 长度较短且不含字母（如 1234567）可能是地块价格或其他 ID，跳过
		if !hasLetter && len(sha) < 10 {
			return sha
		}

		displaySHA := sha
		if len(sha) > 7 {
			displaySHA = sha[:7]
		}
		return fmt.Sprintf("[%s](%s/commit/%s)", displaySHA, repoUrl, sha)
	})

	// 2. 处理 Issue/PR 引用 (#123)
	text = issueRegex.ReplaceAllStringFunc(text, func(match string) string {
		idx := strings.Index(match, "#")
		if idx == -1 {
			return match
		}
		prefix := match[:idx]
		number := match[idx+1:]
		return fmt.Sprintf("%s[#%s](%s/issues/%s)", prefix, number, repoUrl, number)
	})

	return text
}

// containsMarkdownList 检测字符串中是否包含 markdown 无序或有序列表语法
// 匹配行首（或 \n 后）有零或多个空格/制表符，后跟 - * + 或 数字. 及空格
var markdownListRe = regexp.MustCompile(`(?:^|\n)[ \t]*(?:[-*+]|\d+\.)[ \t]+`)

func containsMarkdownList(s string) bool {
	return markdownListRe.MatchString(s)
}

// ProcessCommitMessage 处理提交信息，转换 emoji、高亮 Conventional Commit 前缀，并转换 SHA/Issue 为链接
func ProcessCommitMessage(msg string, repoUrl string) string {
	msg = strings.TrimSpace(msg)
	// 1. 转换 Emoji 短代码
	msg = emoji.Sprint(msg)

	// 2. 转换 SHA 和 #Issue (在加粗前处理，避免 Markdown 嵌套冲突)
	if repoUrl != "" {
		msg = Linkify(msg, repoUrl)
	}

	// 3. 高亮 Conventional Commit 并处理格式
	matches := conventionalRegex.FindAllStringIndex(msg, -1)
	if len(matches) == 0 {
		return msg
	}

	var result strings.Builder
	last := 0
	for _, match := range matches {
		start, end := match[0], match[1]

		// 写入上一个匹配到当前匹配之间的内容
		if start > last {
			part := msg[last:start]
			result.WriteString(part)
		}

		// 加粗匹配的前缀
		result.WriteString("**")
		result.WriteString(msg[start:end])
		result.WriteString("**")

		// 确保 prefix 后面有一个空格（解决 feat:xxxx 无法高亮的问题）
		if end < len(msg) && msg[end] != ' ' && msg[end] != '\n' && msg[end] != '\t' {
			result.WriteString(" ")
		}

		last = end
	}
	result.WriteString(msg[last:])

	return result.String()
}

// FormatDuration 格式化耗时为人类可读格式 (Xh Ym Zs)
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%d hour", h))
		if h > 1 {
			parts[len(parts)-1] += "s"
		}
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%d minute", m))
		if m > 1 {
			parts[len(parts)-1] += "s"
		}
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d second", s))
		if s > 1 {
			parts[len(parts)-1] += "s"
		}
	}
	return strings.Join(parts, " ")
}

// htmlToMarkdown 将 HTML 内容转换为飞书卡片 markdown 组件支持的纯 markdown 语法。
// 飞书卡片的 markdown 标签不支持原始 HTML，只支持标准 markdown 语法。
func htmlToMarkdown(s string) string {
	// Step 1: 内联标签转换（顺序重要：先处理内层标签）
	s = convertInlineTags(s)

	// Step 2: 表格转换（单元格内容是已经转换过的 markdown）
	s = convertHTMLTables(s)

	// Step 3: 块级元素转换
	s = convertBlockTags(s)

	// Step 4: 移除残留的 HTML 标签
	s = regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(s, "")

	// Step 5: 清理多余空白
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	return s
}

var (
	reBr         = regexp.MustCompile(`(?is)<br\s*/?>`)
	reStrong     = regexp.MustCompile(`(?is)<(strong|b)\s*>(.*?)</(strong|b)>`)
	reEm         = regexp.MustCompile(`(?is)<(em|i)\s*>(.*?)</(em|i)>`)
	reCode       = regexp.MustCompile(`(?is)<code\s*>(.*?)</code>`)
	reDel        = regexp.MustCompile(`(?is)<(del|s|strike)\s*>(.*?)</(del|s|strike)>`)
	reA          = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']*)["'][^>]*>(.*?)</a>`)
	reImgAltSrc  = regexp.MustCompile(`(?is)<img\s+[^>]*alt=["']([^"']*)["'][^>]*src=["']([^"']*)["'][^>]*/?>`)
	reImgSrcAlt  = regexp.MustCompile(`(?is)<img\s+[^>]*src=["']([^"']*)["'][^>]*alt=["']([^"']*)["'][^>]*/?>`)
	reImgSrcOnly = regexp.MustCompile(`(?is)<img\s+[^>]*src=["']([^"']*)["'][^>]*/?>`)
	reTable      = regexp.MustCompile(`(?is)<table.*?>(.*?)</table>`)
	reTr         = regexp.MustCompile(`(?is)<tr.*?>(.*?)</tr>`)
	reTd         = regexp.MustCompile(`(?is)<t[dh].*?>(.*?)</t[dh]>`)
	reP          = regexp.MustCompile(`(?is)<p\s*>(.*?)</p>`)
	reHeading    = regexp.MustCompile(`(?is)<h([1-6])\s*>(.*?)</h[1-6]>`)
	reLi         = regexp.MustCompile(`(?is)<li\s*>(.*?)</li>`)
	reBq         = regexp.MustCompile(`(?is)<blockquote\s*>(.*?)</blockquote>`)
	reHr         = regexp.MustCompile(`(?is)<hr\s*/?>`)
)

func convertInlineTags(s string) string {
	s = reBr.ReplaceAllString(s, "\n")
	s = reStrong.ReplaceAllString(s, "**$2**")
	s = reEm.ReplaceAllString(s, "*$2*")
	s = reCode.ReplaceAllString(s, "`$1`")
	s = reDel.ReplaceAllString(s, "~~$2~~")
	s = reA.ReplaceAllString(s, "[$2]($1)")
	s = reImgAltSrc.ReplaceAllString(s, "$1")
	s = reImgSrcAlt.ReplaceAllString(s, "$2")
	s = reImgSrcOnly.ReplaceAllString(s, "[image]($1)")
	return s
}

func convertHTMLTables(s string) string {
	return reTable.ReplaceAllStringFunc(s, func(match string) string {
		var rows [][]string
		for _, trMatch := range reTr.FindAllStringSubmatch(match, -1) {
			var cells []string
			for _, tdMatch := range reTd.FindAllStringSubmatch(trMatch[1], -1) {
				cells = append(cells, strings.TrimSpace(tdMatch[1]))
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
		}
		if len(rows) == 0 {
			return ""
		}

		maxCols := 0
		for _, row := range rows {
			if len(row) > maxCols {
				maxCols = len(row)
			}
		}

		// 单列表格 → 项目列表
		if maxCols == 1 {
			var items []string
			for _, row := range rows {
				items = append(items, "• "+row[0])
			}
			return strings.Join(items, "\n")
		}

		// 多列表格 → markdown 表格
		var lines []string
		for i, row := range rows {
			for len(row) < maxCols {
				row = append(row, "")
			}
			lines = append(lines, "| "+strings.Join(row, " | ")+" |")
			if i == 0 {
				seps := make([]string, maxCols)
				for j := range seps {
					seps[j] = "---"
				}
				lines = append(lines, "| "+strings.Join(seps, " | ")+" |")
			}
		}
		return strings.Join(lines, "\n")
	})
}

func convertBlockTags(s string) string {
	s = reP.ReplaceAllString(s, "$1\n\n")
	s = reHeading.ReplaceAllStringFunc(s, func(m string) string {
		match := reHeading.FindStringSubmatch(m)
		if len(match) > 2 {
			return "\n**" + strings.TrimSpace(match[2]) + "**\n"
		}
		return m
	})
	s = reLi.ReplaceAllString(s, "- $1\n")
	s = reBq.ReplaceAllString(s, "> $1")
	s = reHr.ReplaceAllString(s, "\n---\n")
	return s
}

// ProcessGithubMarkdown 转换 GitHub Markdown 为飞书卡片 Markdown，并提取折叠内容
func ProcessGithubMarkdown(s string) (text string, foldable string) {
	if s == "" {
		return "", ""
	}

	// 1. 预处理 Mermaid
	s = strings.ReplaceAll(s, "```mermaid", "```")

	// 2. 提取 <details> <summary> 折叠内容
	reDetails := regexp.MustCompile(`(?is)<details.*?>\s*<summary.*?>(.*?)</summary>(.*?)</details>`)
	var foldables []string

	processed := reDetails.ReplaceAllStringFunc(s, func(m string) string {
		match := reDetails.FindStringSubmatch(m)
		if len(match) > 2 {
			title := strings.TrimSpace(match[1])
			title = regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(title, "")

			content := strings.TrimSpace(match[2])
			content = htmlToMarkdown(content)
			content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")

			foldables = append(foldables, fmt.Sprintf("**%s**\n%s", title, strings.TrimSpace(content)))
		}
		return ""
	})

	// 3. HTML 转 Markdown
	processed = htmlToMarkdown(processed)
	processed = strings.TrimSpace(processed)

	// 4. 安全截断
	text = SafeText(processed, 50000)
	foldable = SafeText(strings.Join(foldables, "\n\n"), 50000)

	return text, foldable
}

// GetDiffOnlyAdded 生成仅包含新增内容的 Diff
func GetDiffOnlyAdded(old, new string) string {
	if old == "" {
		return new
	}

	oldLines := strings.Split(old, "\n")
	oldMap := make(map[string]bool)
	for _, l := range oldLines {
		oldMap[l] = true
	}

	newLines := strings.Split(new, "\n")
	var diff []string
	for _, l := range newLines {
		if !oldMap[l] {
			diff = append(diff, "+ "+l)
		}
	}

	if len(diff) == 0 {
		return ""
	}
	return strings.Join(diff, "\n")
}

var coAuthorRegex = regexp.MustCompile(`(?im)^Co-authored-by:\s*(.+?)\s*[<＜](.+?)[>＞]`)

type AuthorInfo struct {
	Name   string
	Login  string
	Avatar string
}

// parseCoAuthors 解析提交信息中的共同作者
func parseCoAuthors(msg string) []AuthorInfo {
	matches := coAuthorRegex.FindAllStringSubmatch(msg, -1)
	var authors []AuthorInfo
	for _, m := range matches {
		if len(m) > 2 {
			name := strings.TrimSpace(m[1])
			email := strings.TrimSpace(m[2])
			login := ""
			// 1. 尝试从 GitHub noreply 邮箱提取 login
			if strings.HasSuffix(email, "@users.noreply.github.com") {
				parts := strings.Split(email, "@")
				if len(parts) > 0 {
					loginParts := strings.Split(parts[0], "+")
					login = loginParts[len(loginParts)-1]
				}
			}
			// 2. 如果提取不到，且名字不含空格，尝试把名字当作 login
			if login == "" && !strings.Contains(name, " ") {
				login = name
			}

			// 3. 针对已知的 AI service 或 Bot 的猜测 (仅限 GitHub 官方路径成果)
			if login == "" {
				if strings.Contains(email, "@anthropic.com") {
					login = "Claude"
				} else if strings.Contains(email, "@openai.com") {
					login = "ChatGPT"
				} else if strings.Contains(email, "bot") || strings.Contains(name, "Bot") {
					login = "bot"
				}
			}

			// 4. 统一使用 GitHub 提供的头像
			avatar := ""
			if login != "" {
				avatar = fmt.Sprintf("https://avatars.githubusercontent.com/%s", login)
			}

			authors = append(authors, AuthorInfo{Name: name, Login: login, Avatar: avatar})
		}
	}
	return authors
}
