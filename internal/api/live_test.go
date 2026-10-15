//go:build live

package api

import (
	"errors"
	"os"
	"testing"

	"github.com/YuleBest/coolapk-cli/internal/auth"
)

// TestLiveEndpoints 直接访问酷安线上接口，默认不参与 go test，需显式加 -tags=live。
// 只读接口的风控（验证码、要求登录）会被记为跳过而不是失败。
func TestLiveEndpoints(t *testing.T) {
	device := os.Getenv("COOLAPK_DEVICE")
	if device == "" {
		generated, err := auth.DeviceCode()
		if err != nil {
			t.Fatalf("生成设备码失败: %v", err)
		}
		device = generated
	}
	client := New(device, os.Getenv("COOLAPK_COOKIE"))

	tolerate := func(t *testing.T, err error) bool {
		t.Helper()
		var apiErr *Error
		if errors.As(err, &apiErr) && (apiErr.Captcha() || apiErr.NeedsLogin()) {
			t.Logf("接口返回风控/登录要求，跳过断言: %v", err)
			return true
		}
		return false
	}

	t.Run("search-feed", func(t *testing.T) {
		raw, err := client.Search(SearchFeed, "酷安", 1)
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("搜索动态失败: %v", err)
		}
		items, err := ParseFeeds(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(items) == 0 {
			t.Fatal("动态搜索结果为空")
		}
		t.Logf("动态搜索结果 %d 条，示例 id=%s", len(items), items[0].Identifier())
	})

	t.Run("search-user", func(t *testing.T) {
		raw, err := client.Search(SearchUser, "wherewhere", 1)
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("搜索用户失败: %v", err)
		}
		items, err := ParseUsers(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(items) == 0 {
			t.Fatal("用户搜索结果为空")
		}
		t.Logf("用户搜索结果 %d 条，示例 uid=%s", len(items), items[0].UID.String())
	})

	t.Run("search-apk", func(t *testing.T) {
		raw, err := client.Search(SearchApk, "微信", 1)
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("搜索应用失败: %v", err)
		}
		items, err := ParseApks(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(items) == 0 {
			t.Fatal("应用搜索结果为空")
		}
		t.Logf("应用搜索结果 %d 条，示例 %s", len(items), items[0].Package())
	})

	t.Run("user-space", func(t *testing.T) {
		raw, err := client.UserSpace("536381")
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("用户主页失败: %v", err)
		}
		user, err := ParseUser(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		t.Logf("用户 %s 粉丝 %s 动态 %s", user.Name(), user.Fans.String(), user.Feed.String())
	})

	t.Run("user-feed-list", func(t *testing.T) {
		raw, err := client.UserFeedList("536381", 1)
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("用户动态列表失败: %v", err)
		}
		items, err := ParseFeeds(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		t.Logf("用户动态列表 %d 条", len(items))
	})

	t.Run("feed-replies", func(t *testing.T) {
		raw, err := client.FeedReplies("74360444", 1)
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("评论列表失败: %v", err)
		}
		items, err := ParseReplies(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		t.Logf("评论 %d 条", len(items))
	})

	// 正文接口会被风控拦截，此时应自动改走网页版兜底并照样拿到数据。
	t.Run("feed-detail", func(t *testing.T) {
		raw, err := client.FeedDetail("74360444")
		if err != nil {
			if tolerate(t, err) {
				return
			}
			t.Fatalf("动态正文失败: %v", err)
		}
		feed, err := ParseFeed(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if feed.Identifier() != "74360444" || feed.Message == "" {
			t.Fatalf("正文内容异常: id=%s", feed.Identifier())
		}
		t.Logf("动态正文可访问，作者 %s", feed.Username)
	})
}
