// Command coolapk 是酷安的只读命令行工具。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/YuleBest/coolapk-cli/internal/api"
	"github.com/YuleBest/coolapk-cli/internal/config"
	"github.com/YuleBest/coolapk-cli/internal/render"
)

// version 可通过 -ldflags "-X main.version=…" 覆盖。
var version = "dev"

type usageError struct{ message string }

func (e *usageError) Error() string { return e.message }

func usagef(format string, args ...any) error {
	return &usageError{message: fmt.Sprintf(format, args...)}
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	root := newRootCmd()
	root.SetArgs(args)
	if _, _, err := root.Find(args); err != nil {
		fmt.Fprintln(os.Stderr, "用法错误:", err)
		return 2
	}
	if err := root.Execute(); err != nil {
		var usage *usageError
		if errors.As(err, &usage) {
			fmt.Fprintln(os.Stderr, "用法错误:", usage.message)
			fmt.Fprintln(os.Stderr, "运行 `coolapk --help` 查看用法。")
			return 2
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type app struct {
	jsonOut bool
	noColor bool
	client  *api.Client
}

// api 惰性加载配置，只有真正访问接口的命令才会读写配置文件。
func (a *app) api() (*api.Client, error) {
	if a.client != nil {
		return a.client, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Warning != "" {
		fmt.Fprintln(os.Stderr, "提示:", cfg.Warning)
	}
	a.client = api.New(cfg.Device, cfg.Cookie)
	return a.client, nil
}

func newRootCmd() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:   "coolapk",
		Short: "酷安命令行工具（只读）",
		Long: strings.TrimSpace(`
coolapk 是酷安的只读命令行工具，支持搜索动态/用户/应用、查看动态正文与评论区、查看用户主页。
默认匿名访问，把已登录客户端的 Cookie 写入配置文件可提高风控通过率。`),
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usagef("%s", err) })
	root.PersistentFlags().BoolVar(&a.jsonOut, "json", false, "输出接口原始 JSON 数据")
	root.PersistentFlags().BoolVar(&a.noColor, "no-color", false, "禁用彩色输出")
	root.AddCommand(a.searchCommand(), a.feedCommand(), a.userCommand(), versionCommand())
	return root
}

func (a *app) searchCommand() *cobra.Command {
	var feedQuery, userQuery, apkQuery string
	var page, limit int
	command := &cobra.Command{
		Use:   "search",
		Short: "搜索动态、用户或应用",
		Long: strings.TrimSpace(`
按 --feed / --user / --apk 三选一搜索，结果默认输出文本，加 --json 输出原始数据。
示例:
  coolapk search --feed "小米13"
  coolapk search --user wherewhere --page 2
  coolapk search --apk 微信 --limit 5`),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("search 不接受位置参数，关键词请用 --feed/--user/--apk 传入")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			kind, query, err := pickSearchKind(feedQuery, userQuery, apkQuery)
			if err != nil {
				return err
			}
			if page < 1 {
				return usagef("--page 需要大于 0")
			}
			if limit < 1 {
				return usagef("--limit 需要大于 0")
			}
			client, err := a.api()
			if err != nil {
				return err
			}
			raw, err := client.Search(kind, query, page)
			if err != nil {
				return describe(err)
			}
			out := cmd.OutOrStdout()
			if a.jsonOut {
				return writeJSON(out, raw)
			}
			printer := render.New(out, render.ColorEnabled(out, a.noColor))
			switch kind {
			case api.SearchFeed:
				items, err := api.ParseFeeds(raw)
				if err != nil {
					return err
				}
				printer.SearchFeeds(items, page, limit)
			case api.SearchUser:
				items, err := api.ParseUsers(raw)
				if err != nil {
					return err
				}
				printer.SearchUsers(items, page, limit)
			case api.SearchApk:
				items, err := api.ParseApks(raw)
				if err != nil {
					return err
				}
				printer.SearchApks(items, page, limit)
			}
			return nil
		},
	}
	command.Flags().StringVar(&feedQuery, "feed", "", "按动态搜索")
	command.Flags().StringVar(&userQuery, "user", "", "按用户搜索")
	command.Flags().StringVar(&apkQuery, "apk", "", "按应用搜索")
	command.Flags().IntVar(&page, "page", 1, "页码，从 1 开始")
	command.Flags().IntVar(&limit, "limit", 20, "本页最多显示多少条")
	return command
}

func pickSearchKind(feed, user, apk string) (api.SearchKind, string, error) {
	candidates := []struct {
		kind  api.SearchKind
		value string
	}{
		{api.SearchFeed, strings.TrimSpace(feed)},
		{api.SearchUser, strings.TrimSpace(user)},
		{api.SearchApk, strings.TrimSpace(apk)},
	}
	var kind api.SearchKind
	var query string
	for _, candidate := range candidates {
		if candidate.value == "" {
			continue
		}
		if query != "" {
			return "", "", usagef("--feed/--user/--apk 只能选一个")
		}
		kind, query = candidate.kind, candidate.value
	}
	if query == "" {
		return "", "", usagef("需要指定搜索类型：--feed、--user 或 --apk")
	}
	return kind, query, nil
}

func (a *app) feedCommand() *cobra.Command {
	var commentPage, commentLimit int
	var noComments bool
	command := &cobra.Command{
		Use:   "feed <动态ID>",
		Short: "查看动态正文与评论区",
		Long: strings.TrimSpace(`
查看单条动态的正文和评论区。默认输出正文加评论第一页，
--no-comments 只看正文，--comment-page/--comment-limit 控制评论区。
示例:
  coolapk feed 74360444
  coolapk feed 74360444 --comment-page 2 --comment-limit 10
  coolapk feed 74360444 --no-comments --json`),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usagef("需要一个动态 ID，例如 coolapk feed 74360444")
			}
			if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
				return usagef("动态 ID 必须是数字：%q", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentPage < 1 {
				return usagef("--comment-page 需要大于 0")
			}
			if commentLimit < 1 {
				return usagef("--comment-limit 需要大于 0")
			}
			id := args[0]
			client, err := a.api()
			if err != nil {
				return err
			}

			detailRaw, detailErr := client.FeedDetail(id)
			var repliesRaw json.RawMessage
			var repliesErr error
			if !noComments {
				repliesRaw, repliesErr = client.FeedReplies(id, commentPage)
			}

			out := cmd.OutOrStdout()
			if a.jsonOut {
				if err := writeJSON(out, map[string]json.RawMessage{
					"feed":     detailRaw,
					"comments": repliesRaw,
				}); err != nil {
					return err
				}
			} else {
				printer := render.New(out, render.ColorEnabled(out, a.noColor))
				if detailErr == nil {
					feed, err := api.ParseFeed(detailRaw)
					if err != nil {
						return err
					}
					printer.FeedDetail(feed)
				}
				if !noComments && repliesErr == nil {
					replies, err := api.ParseReplies(repliesRaw)
					if err != nil {
						return err
					}
					printer.Comments(replies, commentPage, commentLimit)
				}
			}

			switch {
			case detailErr != nil && repliesErr != nil:
				return fmt.Errorf("动态正文与评论区都获取失败: %w", describe(detailErr))
			case detailErr != nil:
				return fmt.Errorf("动态正文获取失败: %w", describe(detailErr))
			case repliesErr != nil:
				return fmt.Errorf("评论区获取失败: %w", describe(repliesErr))
			}
			return nil
		},
	}
	command.Flags().IntVar(&commentPage, "comment-page", 1, "评论页码，从 1 开始")
	command.Flags().IntVar(&commentLimit, "comment-limit", 20, "本页最多显示多少条评论")
	command.Flags().BoolVar(&noComments, "no-comments", false, "只输出动态正文")
	return command
}

func (a *app) userCommand() *cobra.Command {
	var page, limit int
	command := &cobra.Command{
		Use:   "user <uid>",
		Short: "查看用户主页",
		Long: strings.TrimSpace(`
查看用户资料（粉丝、关注、动态数、等级、简介等）与动态列表。
uid 是数字，可先用 coolapk search --user <关键词> 查到。
示例:
  coolapk user 536381
  coolapk user 536381 --page 2 --limit 10`),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usagef("需要一个用户 uid，例如 coolapk user 536381")
			}
			if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
				return usagef("uid 必须是数字：%q（可用 coolapk search --user <关键词> 查询）", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if page < 1 {
				return usagef("--page 需要大于 0")
			}
			if limit < 1 {
				return usagef("--limit 需要大于 0")
			}
			uid := args[0]
			client, err := a.api()
			if err != nil {
				return err
			}
			userRaw, err := client.UserSpace(uid)
			if err != nil {
				return fmt.Errorf("用户资料获取失败: %w", describe(err))
			}
			feedsRaw, feedsErr := client.UserFeedList(uid, page)

			out := cmd.OutOrStdout()
			if a.jsonOut {
				if err := writeJSON(out, map[string]json.RawMessage{
					"user":  userRaw,
					"feeds": feedsRaw,
				}); err != nil {
					return err
				}
			} else {
				user, err := api.ParseUser(userRaw)
				if err != nil {
					return err
				}
				var feeds []api.FeedItem
				if feedsErr == nil {
					feeds, err = api.ParseFeeds(feedsRaw)
					if err != nil {
						return err
					}
				}
				render.New(out, render.ColorEnabled(out, a.noColor)).UserProfile(user, feeds, page, limit)
			}
			if feedsErr != nil {
				return fmt.Errorf("动态列表获取失败: %w", describe(feedsErr))
			}
			return nil
		},
	}
	command.Flags().IntVar(&page, "page", 1, "动态列表页码，从 1 开始")
	command.Flags().IntVar(&limit, "limit", 20, "本页最多显示多少条动态")
	return command
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印版本",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("version 不接受参数")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "coolapk %s\n", version)
			return nil
		},
	}
}

// describe 把接口错误翻译成可操作的提示。
func describe(err error) error {
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	configPath, pathErr := config.Path()
	if pathErr != nil {
		configPath = "~/.config/coolapk/config.json"
	}
	switch {
	case apiErr.Captcha():
		return fmt.Errorf("触发酷安风控，需要人机验证（%s）。\n该验证与登录态无关，配置 Cookie 也不会绕过，可稍后重试", apiErr.Error())
	case apiErr.NeedsLogin():
		return fmt.Errorf("该接口需要登录（%s）。\n可在 %s 的 cookie 字段填入已登录 Cookie", apiErr.Error(), configPath)
	}
	return fmt.Errorf("酷安接口返回错误: %w", apiErr)
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
