package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/YuleBest/coolapk-cli/internal/api"
)

func TestHTMLToText(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"纯文本", "普通正文", "普通正文"},
		{
			"链接",
			`下载地址：<a class="feed-link-url" href="https://www.oppo.cn/thread-1">查看链接</a>）`,
			"下载地址：查看链接 <https://www.oppo.cn/thread-1>）",
		},
		{"换行", "第一行<br/>第二行", "第一行\n第二行"},
		{"段落", "<p>第一段</p><p>第二段</p>", "第一段\n第二段"},
		{"实体", "&lt;测试&gt; &amp; 符号", "<测试> & 符号"},
		{"图片", `<img src="https://img/x.jpg"/>`, "图片 https://img/x.jpg"},
		{"站内链接", `<a href="/t/酷安?type=0">#酷安#</a> 正文`, "#酷安# 正文"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := htmlToText(testCase.source); got != testCase.want {
				t.Fatalf("htmlToText() = %q, 期望 %q", got, testCase.want)
			}
		})
	}
}

func TestSummarizeTruncates(t *testing.T) {
	message := strings.Repeat("酷", 100)
	got := summarize(message, 10)
	if got != strings.Repeat("酷", 10)+"…" {
		t.Fatalf("summarize() = %q", got)
	}
	if got := summarize("多行\n文本 <a href=\"https://x\">链接</a>", 80); strings.Contains(got, "\n") {
		t.Fatalf("摘要不应包含换行: %q", got)
	}
}

func TestSearchFeedsRendersCoreFields(t *testing.T) {
	var buffer bytes.Buffer
	printer := New(&buffer, false)
	printer.SearchFeeds([]api.FeedItem{{
		EntityID:     "74384253",
		Title:        "水墨青竹的动态",
		Message:      "手持一加13，ColorOS 16 解锁说明",
		Username:     "水墨青竹",
		DatelineText: "2分钟前",
		LikeNum:      "1",
		ReplyNum:     "0",
		DeviceTitle:  "REDMI K90 Pro Max",
		PicArr:       []string{"https://img/1.jpg"},
	}}, 1, 20)

	out := buffer.String()
	for _, want := range []string{"动态搜索结果：第 1 页，返回 1 条", "74384253", "水墨青竹", "2分钟前", "赞 1", "评论 0", "REDMI K90 Pro Max", "图片 1 张"} {
		if !strings.Contains(out, want) {
			t.Fatalf("输出缺少 %q:\n%s", want, out)
		}
	}
}

func TestFeedDetailGolden(t *testing.T) {
	var buffer bytes.Buffer
	printer := New(&buffer, false)
	printer.FeedDetail(api.FeedItem{
		EntityID:     "74360444",
		Title:        "测试动态",
		Message:      "第一段<br/>查看链接：<a href=\"https://example.com/a\">点这里</a>",
		Username:     "wherewhere",
		UID:          "536381",
		DatelineText: "2天前",
		LikeNum:      "17",
		ReplyNum:     "3",
		ForwardNum:   "1",
		FavNum:       "0",
		DeviceTitle:  "REDMI K90 Pro Max",
		PicArr:       []string{"https://img/1.jpg"},
	})

	want := strings.TrimSpace(`
动态 74360444
wherewhere · uid 536381 · 2天前 · REDMI K90 Pro Max
测试动态
赞 17 · 评论 3 · 转发 1 · 收藏 0

第一段
查看链接：点这里 <https://example.com/a>

图片 https://img/1.jpg
`)
	if got := strings.TrimSpace(buffer.String()); got != want {
		t.Fatalf("正文输出不符:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCommentsRenderSubRepliesAndMore(t *testing.T) {
	var buffer bytes.Buffer
	printer := New(&buffer, false)
	printer.Comments([]api.ReplyItem{{
		Username:      "kexer1998",
		Message:       "主评论正文",
		DatelineText:  "14小时前",
		LikeNum:       "263",
		ReplyNum:      "12",
		IsFeedAuthor:  "0",
		ReplyRows:     []api.ReplyItem{{Username: "K1ngMaster", Message: "子评论正文", LikeNum: "0"}},
		ReplyRowsMore: "7",
	}}, 1, 20)

	out := buffer.String()
	for _, want := range []string{"评论区：第 1 页，返回 1 条", "kexer1998", "主评论正文", "└ K1ngMaster", "子评论正文", "（另有 7 条回复）"} {
		if !strings.Contains(out, want) {
			t.Fatalf("输出缺少 %q:\n%s", want, out)
		}
	}
}

func TestUserProfileGolden(t *testing.T) {
	var buffer bytes.Buffer
	printer := New(&buffer, false)
	printer.UserProfile(api.UserItem{
		UID:        "536381",
		Username:   "wherewhere",
		Bio:        "简介第一行\n简介第二行",
		Fans:       "3198",
		Follow:     "54",
		Feed:       "1178",
		BeLikeNum:  "21693",
		Level:      "8",
		Experience: "366076",
		RegDate:    "1456134898",
		Province:   "安徽",
		City:       "芜湖",
	}, []api.FeedItem{{
		EntityID:     "74384253",
		Title:        "水墨青竹的动态",
		Username:     "水墨青竹",
		DatelineText: "2分钟前",
		LikeNum:      "1",
		ReplyNum:     "0",
	}}, 1, 20)

	out := buffer.String()
	for _, want := range []string{"wherewhere [536381]", "粉丝 3198 · 关注 54 · 动态 1178 · 获赞 21693", "等级 8 · 经验 366076", "地区 安徽 芜湖", "注册于 2016-02-22", "简介第一行", "动态列表：第 1 页，返回 1 条", "74384253"} {
		if !strings.Contains(out, want) {
			t.Fatalf("输出缺少 %q:\n%s", want, out)
		}
	}
}

func TestColorDisabledByDefaultInTests(t *testing.T) {
	var buffer bytes.Buffer
	if ColorEnabled(&buffer, false) {
		t.Fatal("非终端输出不应启用颜色")
	}
	if ColorEnabled(&buffer, true) {
		t.Fatal("显式禁用时不应启用颜色")
	}
}

func TestLimitCutsPage(t *testing.T) {
	var buffer bytes.Buffer
	printer := New(&buffer, false)
	printer.SearchUsers([]api.UserItem{
		{UID: "1", Username: "a"},
		{UID: "2", Username: "b"},
		{UID: "3", Username: "c"},
	}, 1, 2)
	out := buffer.String()
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("应显示前两条:\n%s", out)
	}
	if strings.Contains(out, "c") {
		t.Fatalf("超出 limit 的条目不显示:\n%s", out)
	}
}
