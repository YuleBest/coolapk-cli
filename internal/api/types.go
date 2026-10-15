package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Text 兼容酷安接口里数字与字符串混用的字段。
type Text string

func (t *Text) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*t = ""
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = Text(value)
		return nil
	}
	*t = Text(trimmed)
	return nil
}

func (t Text) String() string { return string(t) }

// Int64 解析数字字段，字段为空或非数字时返回错误。
func (t Text) Int64() (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(string(t)), 10, 64)
}

// FeedItem 同时用于搜索结果与动态列表。
type FeedItem struct {
	EntityID     Text        `json:"entityId"`
	ID           Text        `json:"id"`
	Title        string      `json:"title"`
	Message      string      `json:"message"`
	Username     string      `json:"username"`
	UID          Text        `json:"uid"`
	Dateline     Text        `json:"dateline"`
	DatelineText string      `json:"dateline_text"`
	LikeNum      Text        `json:"likenum"`
	ReplyNum     Text        `json:"replynum"`
	ForwardNum   Text        `json:"forwardnum"`
	FavNum       Text        `json:"favnum"`
	DeviceTitle  string      `json:"device_title"`
	FeedTypeName string      `json:"feedTypeName"`
	Pic          string      `json:"pic"`
	PicArr       []string    `json:"picArr"`
	ReplyRows    []ReplyItem `json:"replyRows"`
}

// Identifier 返回动态的真实 ID，优先 entityId。
func (f FeedItem) Identifier() string {
	if id := f.EntityID.String(); id != "" {
		return id
	}
	return f.ID.String()
}

// ReplyItem 是评论及其子评论。
type ReplyItem struct {
	ID             Text        `json:"id"`
	UID            Text        `json:"uid"`
	Username       string      `json:"username"`
	Message        string      `json:"message"`
	Dateline       Text        `json:"dateline"`
	DatelineText   string      `json:"dateline_text"`
	LikeNum        Text        `json:"likenum"`
	ReplyNum       Text        `json:"replynum"`
	IsFeedAuthor   Text        `json:"isFeedAuthor"`
	RUsername      string      `json:"rusername"`
	ReplyRows      []ReplyItem `json:"replyRows"`
	ReplyRowsCount Text        `json:"replyRowsCount"`
	ReplyRowsMore  Text        `json:"replyRowsMore"`
}

// UserItem 是用户资料。
type UserItem struct {
	UID             Text   `json:"uid"`
	Username        string `json:"username"`
	DisplayUsername string `json:"displayUsername"`
	Bio             string `json:"bio"`
	Fans            Text   `json:"fans"`
	Follow          Text   `json:"follow"`
	Feed            Text   `json:"feed"`
	ReplyNum        Text   `json:"replyNum"`
	Level           Text   `json:"level"`
	Experience      Text   `json:"experience"`
	BeLikeNum       Text   `json:"be_like_num"`
	RegDate         Text   `json:"regdate"`
	City            string `json:"city"`
	Province        string `json:"province"`
	IsDeveloper     Text   `json:"isDeveloper"`
	VerifyTitle     string `json:"verify_title"`
}

// Name 返回展示用昵称。
func (u UserItem) Name() string {
	if name := strings.TrimSpace(u.DisplayUsername); name != "" {
		return name
	}
	return u.Username
}

// ApkItem 是应用搜索结果。
type ApkItem struct {
	Title         string `json:"title"`
	ApkName       string `json:"apkname"`
	PackageName   string `json:"packageName"`
	Version       string `json:"version"`
	VersionName   string `json:"apkversionname"`
	Score         Text   `json:"score"`
	DownCount     Text   `json:"downCount"`
	CommentCount  Text   `json:"commentnum"`
	FollowCount   Text   `json:"followCount"`
	DeveloperName string `json:"developername"`
	CatName       string `json:"catName"`
	Description   string `json:"description"`
}

// Package 返回包名，兼容 apkname 与 packageName 两个字段。
func (a ApkItem) Package() string {
	if name := strings.TrimSpace(a.PackageName); name != "" {
		return name
	}
	return strings.TrimSpace(a.ApkName)
}

func ParseFeeds(raw json.RawMessage) ([]FeedItem, error) {
	return parseList[FeedItem](raw, "动态")
}

func ParseUsers(raw json.RawMessage) ([]UserItem, error) {
	return parseList[UserItem](raw, "用户")
}

func ParseApks(raw json.RawMessage) ([]ApkItem, error) {
	return parseList[ApkItem](raw, "应用")
}

func ParseReplies(raw json.RawMessage) ([]ReplyItem, error) {
	return parseList[ReplyItem](raw, "评论")
}

func ParseFeed(raw json.RawMessage) (FeedItem, error) {
	if isNull(raw) {
		return FeedItem{}, errors.New("未找到该动态")
	}
	var item FeedItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return FeedItem{}, fmt.Errorf("解析动态失败: %w", err)
	}
	return item, nil
}

func ParseUser(raw json.RawMessage) (UserItem, error) {
	if isNull(raw) {
		return UserItem{}, errors.New("未找到该用户")
	}
	var item UserItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return UserItem{}, fmt.Errorf("解析用户资料失败: %w", err)
	}
	return item, nil
}

func parseList[T any](raw json.RawMessage, label string) ([]T, error) {
	if isNull(raw) {
		return nil, nil
	}
	var items []T
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("解析%s列表失败: %w", label, err)
	}
	return items, nil
}

func isNull(raw json.RawMessage) bool {
	return len(raw) == 0 || strings.TrimSpace(string(raw)) == "null"
}
