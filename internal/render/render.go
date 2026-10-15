// Package render 把酷安接口数据渲染成终端可读文本。
package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/YuleBest/coolapk-cli/internal/api"
)

const (
	summaryLimit  = 80
	subReplyLimit = 5
)

// Printer 负责输出格式与颜色。
type Printer struct {
	out   io.Writer
	color bool
}

func New(out io.Writer, color bool) *Printer {
	return &Printer{out: out, color: color}
}

// ColorEnabled 在显式禁用、设置了 NO_COLOR 或输出不是终端时返回 false。
func ColorEnabled(out io.Writer, disabled bool) bool {
	if disabled || os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// SearchFeeds 渲染动态搜索结果。
func (p *Printer) SearchFeeds(items []api.FeedItem, page, limit int) {
	p.line("%s", p.bold(fmt.Sprintf("动态搜索结果：第 %d 页，返回 %d 条", page, len(items))))
	if len(items) == 0 {
		p.line("没有找到结果。")
		return
	}
	p.feedList(items, limit)
}

// SearchUsers 渲染用户搜索结果。
func (p *Printer) SearchUsers(items []api.UserItem, page, limit int) {
	p.line("%s", p.bold(fmt.Sprintf("用户搜索结果：第 %d 页，返回 %d 条", page, len(items))))
	if len(items) == 0 {
		p.line("没有找到结果。")
		return
	}
	for index, item := range items {
		if index >= limit {
			break
		}
		p.line("")
		p.line("%s %s %s", p.cyan(fmt.Sprintf("%2d.", index+1)), p.bold(item.Name()), p.dim("["+item.UID.String()+"]"))
		if stats := joinStats("粉丝", item.Fans.String(), "关注", item.Follow.String(), "动态", item.Feed.String(),
			"等级", item.Level.String(), "经验", item.Experience.String()); stats != "" {
			p.line("    %s", p.dim(stats))
		}
		if bio := summarize(item.Bio, summaryLimit); bio != "" {
			p.line("    %s", bio)
		}
	}
}

// SearchApks 渲染应用搜索结果。
func (p *Printer) SearchApks(items []api.ApkItem, page, limit int) {
	p.line("%s", p.bold(fmt.Sprintf("应用搜索结果：第 %d 页，返回 %d 条", page, len(items))))
	if len(items) == 0 {
		p.line("没有找到结果。")
		return
	}
	for index, item := range items {
		if index >= limit {
			break
		}
		p.line("")
		name := strings.TrimSpace(item.Title)
		if name == "" {
			name = item.Package()
		}
		version := firstNonEmpty(item.Version, item.VersionName)
		if version != "" {
			version = " v" + strings.TrimPrefix(version, "v")
		}
		p.line("%s %s%s %s", p.cyan(fmt.Sprintf("%2d.", index+1)), p.bold(name), version, p.dim("("+item.Package()+")"))
		if stats := joinStats("评分", item.Score.String(), "下载", item.DownCount.String(),
			"评论", item.CommentCount.String(), "关注", item.FollowCount.String()); stats != "" {
			p.line("    %s", p.dim(stats))
		}
		meta := []string{}
		if item.DeveloperName != "" {
			meta = append(meta, "开发者 "+item.DeveloperName)
		}
		if item.CatName != "" {
			meta = append(meta, item.CatName)
		}
		if len(meta) > 0 {
			p.line("    %s", p.dim(strings.Join(meta, " · ")))
		}
		if description := summarize(item.Description, summaryLimit); description != "" {
			p.line("    %s", description)
		}
	}
}

// FeedDetail 渲染单条动态正文。
func (p *Printer) FeedDetail(feed api.FeedItem) {
	p.line("%s %s", p.bold("动态"), p.dim(feed.Identifier()))
	meta := []string{}
	if feed.Username != "" {
		meta = append(meta, feed.Username)
	}
	if uid := feed.UID.String(); uid != "" {
		meta = append(meta, "uid "+uid)
	}
	if label := timeLabel(feed.DatelineText, feed.Dateline); label != "" {
		meta = append(meta, label)
	}
	if feed.DeviceTitle != "" {
		meta = append(meta, feed.DeviceTitle)
	}
	if len(meta) > 0 {
		p.line("%s", p.dim(strings.Join(meta, " · ")))
	}
	if title := strings.TrimSpace(feed.Title); title != "" && title != feed.Username+"的动态" {
		p.line("%s", p.bold(title))
	}
	if stats := joinStats("赞", feed.LikeNum.String(), "评论", feed.ReplyNum.String(),
		"转发", feed.ForwardNum.String(), "收藏", feed.FavNum.String()); stats != "" {
		p.line("%s", p.dim(stats))
	}
	if message := htmlToText(feed.Message); message != "" {
		p.line("")
		p.line("%s", message)
	}
	pictures := feed.PicArr
	if len(pictures) == 0 && feed.Pic != "" {
		pictures = []string{feed.Pic}
	}
	if len(pictures) > 0 {
		p.line("")
		for _, picture := range pictures {
			p.line("%s %s", p.dim("图片"), picture)
		}
	}
}

// Comments 渲染评论区，子评论缩进展示。
func (p *Printer) Comments(replies []api.ReplyItem, page, limit int) {
	p.line("")
	p.line("%s", p.bold(fmt.Sprintf("评论区：第 %d 页，返回 %d 条", page, len(replies))))
	if len(replies) == 0 {
		p.line("暂无评论。")
		return
	}
	for index, reply := range replies {
		if index >= limit {
			break
		}
		p.line("")
		header := p.cyan(fmt.Sprintf("%2d.", index+1)) + " " + p.bold(reply.Username)
		if reply.RUsername != "" {
			header += " 回复 " + reply.RUsername
		}
		if reply.IsFeedAuthor.String() != "" && reply.IsFeedAuthor.String() != "0" {
			header += " " + p.yellow("[作者]")
		}
		meta := []string{}
		if label := timeLabel(reply.DatelineText, reply.Dateline); label != "" {
			meta = append(meta, label)
		}
		if stats := joinStats("赞", reply.LikeNum.String(), "回复", reply.ReplyNum.String()); stats != "" {
			meta = append(meta, stats)
		}
		if len(meta) > 0 {
			header += " · " + p.dim(strings.Join(meta, " · "))
		}
		p.line("    %s", header)
		p.writeBody(reply.Message, "    ")

		for subIndex, sub := range reply.ReplyRows {
			if subIndex >= subReplyLimit {
				break
			}
			subHeader := "└ " + p.bold(sub.Username)
			if sub.RUsername != "" {
				subHeader += " 回复 " + sub.RUsername
			}
			if stats := joinStats("赞", sub.LikeNum.String()); stats != "" {
				subHeader += " · " + p.dim(stats)
			}
			p.line("      %s", subHeader)
			p.writeBody(sub.Message, "        ")
		}
		if more, err := reply.ReplyRowsMore.Int64(); err == nil && more > 0 {
			p.line("      %s", p.dim(fmt.Sprintf("（另有 %d 条回复）", more)))
		}
	}
}

// UserProfile 渲染用户资料与动态列表。
func (p *Printer) UserProfile(user api.UserItem, feeds []api.FeedItem, page, limit int) {
	p.line("%s %s", p.bold(user.Name()), p.dim("["+user.UID.String()+"]"))
	if stats := joinStats("粉丝", user.Fans.String(), "关注", user.Follow.String(),
		"动态", user.Feed.String(), "获赞", user.BeLikeNum.String()); stats != "" {
		p.line("%s", p.dim(stats))
	}
	if stats := joinStats("等级", user.Level.String(), "经验", user.Experience.String()); stats != "" {
		p.line("%s", p.dim(stats))
	}
	meta := []string{}
	if region := strings.TrimSpace(user.Province + " " + user.City); region != "" {
		meta = append(meta, "地区 "+region)
	}
	if registered := dateLabel(user.RegDate); registered != "" {
		meta = append(meta, "注册于 "+registered)
	}
	if user.IsDeveloper.String() == "1" {
		meta = append(meta, "开发者")
	}
	if user.VerifyTitle != "" {
		meta = append(meta, user.VerifyTitle)
	}
	if len(meta) > 0 {
		p.line("%s", p.dim(strings.Join(meta, " · ")))
	}
	if bio := htmlToText(user.Bio); strings.TrimSpace(bio) != "" {
		p.line("")
		for _, line := range strings.Split(bio, "\n") {
			p.line("  %s", line)
		}
	}
	p.line("")
	p.line("%s", p.bold(fmt.Sprintf("动态列表：第 %d 页，返回 %d 条", page, len(feeds))))
	if len(feeds) == 0 {
		p.line("暂无动态。")
		return
	}
	p.feedList(feeds, limit)
}

func (p *Printer) feedList(items []api.FeedItem, limit int) {
	for index, item := range items {
		if index >= limit {
			break
		}
		p.line("")
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = summarize(item.Message, 20)
		}
		if title == "" {
			title = "（无正文）"
		}
		p.line("%s %s %s", p.cyan(fmt.Sprintf("%2d.", index+1)), p.bold(title), p.dim("["+item.Identifier()+"]"))
		meta := []string{}
		if item.Username != "" {
			meta = append(meta, item.Username)
		}
		if label := timeLabel(item.DatelineText, item.Dateline); label != "" {
			meta = append(meta, label)
		}
		if item.DeviceTitle != "" {
			meta = append(meta, item.DeviceTitle)
		}
		if len(meta) > 0 {
			p.line("    %s", p.dim(strings.Join(meta, " · ")))
		}
		if stats := joinStats("赞", item.LikeNum.String(), "评论", item.ReplyNum.String(),
			"转发", item.ForwardNum.String()); stats != "" {
			p.line("    %s", p.dim(stats))
		}
		if summary := summarize(item.Message, summaryLimit); summary != "" {
			p.line("    %s", summary)
		}
		if len(item.PicArr) > 0 {
			p.line("    %s", p.dim(fmt.Sprintf("图片 %d 张", len(item.PicArr))))
		}
	}
}

func (p *Printer) writeBody(message, prefix string) {
	body := htmlToText(message)
	if strings.TrimSpace(body) == "" {
		return
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			p.line("")
			continue
		}
		p.line("%s%s", prefix, line)
	}
}

func (p *Printer) line(format string, args ...any) {
	fmt.Fprintf(p.out, format+"\n", args...)
}

func (p *Printer) style(code, text string) string {
	if !p.color || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (p *Printer) bold(text string) string   { return p.style("1", text) }
func (p *Printer) dim(text string) string    { return p.style("2", text) }
func (p *Printer) cyan(text string) string   { return p.style("36", text) }
func (p *Printer) yellow(text string) string { return p.style("33", text) }

func timeLabel(relative string, epoch api.Text) string {
	if relative = strings.TrimSpace(relative); relative != "" {
		return relative
	}
	seconds, err := epoch.Int64()
	if err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).Format("2006-01-02 15:04")
}

func dateLabel(epoch api.Text) string {
	seconds, err := epoch.Int64()
	if err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).Format("2006-01-02")
}

func joinStats(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		if value := strings.TrimSpace(pairs[i+1]); value != "" {
			parts = append(parts, pairs[i]+" "+value)
		}
	}
	return strings.Join(parts, " · ")
}

func summarize(message string, limit int) string {
	text := strings.Join(strings.Fields(htmlToText(message)), " ")
	return truncate(text, limit)
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
