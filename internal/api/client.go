// Package api 封装酷安 V6 只读接口：请求头、签名、响应信封与各端点。
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/YuleBest/coolapk-cli/internal/auth"
)

const (
	defaultBaseURL    = "https://api.coolapk.com"
	defaultWebBaseURL = "https://www.coolapk.com"
	userAgent         = "Dalvik/2.1.0 (Linux; U; Android 16; PKQ110 Build/BP2A.250605.031.A1) +CoolMarket/16.2.0-2604201-universal"
	mobileUserAgent   = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"
	appVersion        = "16.2.0"
	appCode           = "2604201"
	apiVersion        = "16"
	sdkInt            = "36"

	maxResponseBytes = 8 << 20
)

// SearchKind 是搜索类型，对应 /v6/search 的 type 参数。
type SearchKind string

const (
	SearchFeed SearchKind = "feed"
	SearchUser SearchKind = "user"
	SearchApk  SearchKind = "apk"
)

// Client 是酷安接口的最小只读客户端。
type Client struct {
	http       *http.Client
	baseURL    string
	webBaseURL string
	device     string
	cookie     string
	now        func() time.Time
}

// New 创建客户端，device 为空时请求会失败，cookie 可选。
func New(device, cookie string) *Client {
	return &Client{
		http:       &http.Client{Timeout: 20 * time.Second},
		baseURL:    defaultBaseURL,
		webBaseURL: defaultWebBaseURL,
		device:     device,
		cookie:     cookie,
		now:        time.Now,
	}
}

// Error 是酷安在 HTTP 200 下返回的业务错误。
type Error struct {
	Status        int
	Code          int
	Message       string
	MessageStatus string
	MessageExtra  string
}

func (e *Error) Error() string {
	message := e.Message
	if message == "" {
		message = "接口返回未知错误"
	}
	details := make([]string, 0, 3)
	if e.Code != 0 {
		details = append(details, "code="+strconv.Itoa(e.Code))
	}
	if e.Status != 0 && e.Status != e.Code {
		details = append(details, "status="+strconv.Itoa(e.Status))
	}
	if e.MessageStatus != "" {
		details = append(details, "messageStatus="+e.MessageStatus)
	}
	if len(details) == 0 {
		return message
	}
	return fmt.Sprintf("%s (%s)", message, strings.Join(details, ", "))
}

// Captcha 表示接口要求人机验证。v1 不做验证码流程，只向用户说明。
func (e *Error) Captcha() bool {
	if strings.Contains(e.MessageStatus, "captcha") {
		return true
	}
	return e.Code == http.StatusForbidden && strings.Contains(e.Message, "验证码")
}

// TokenExpired 表示 X-App-Token 不被接受，重新签发后可重试。
func (e *Error) TokenExpired() bool { return e.Status == 1004 }

// NeedsLogin 表示接口需要登录态。
func (e *Error) NeedsLogin() bool {
	return e.Status == http.StatusUnauthorized || strings.Contains(e.Message, "登录")
}

// Search 返回搜索结果原始 data，kind 决定 type 参数。
func (c *Client) Search(kind SearchKind, query string, page int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("searchValue", query)
	params.Set("page", strconv.Itoa(page))
	params.Set("showAnonymous", "-1")
	switch kind {
	case SearchFeed:
		params.Set("type", string(SearchFeed))
		params.Set("feedType", "all")
		params.Set("sort", "default")
	case SearchUser:
		params.Set("type", string(SearchUser))
	case SearchApk:
		params.Set("type", string(SearchApk))
	default:
		return nil, fmt.Errorf("未知的搜索类型 %q", kind)
	}
	return c.get("/v6/search", params)
}

// FeedDetail 返回单条动态的原始 data。
//
// 移动端接口会被酷安风控拦住并要求人机验证，此时改走网页版
// www.coolapk.com/feed/<id>，带上 X-Requested-With 与 X-App-Id 时它会直接返回同构的 JSON。
func (c *Client) FeedDetail(id string) (json.RawMessage, error) {
	data, err := c.get("/v6/feed/detail", url.Values{"id": {id}})
	if err == nil {
		return data, nil
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) || !apiErr.Captcha() {
		return nil, err
	}
	webData, webErr := c.getWeb("/feed/" + url.PathEscape(id))
	if webErr != nil {
		return nil, fmt.Errorf("移动端接口要求人机验证，网页版兜底也失败: %w", webErr)
	}
	return webData, nil
}

// FeedReplies 返回动态评论区的原始 data。
func (c *Client) FeedReplies(id string, page int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("id", id)
	params.Set("page", strconv.Itoa(page))
	params.Set("listType", "lastupdate_desc")
	params.Set("discussMode", "1")
	params.Set("feedType", "feed")
	params.Set("blockStatus", "0")
	params.Set("fromFeedAuthor", "0")
	return c.get("/v6/feed/replyList", params)
}

// UserSpace 返回用户主页资料原始 data。
func (c *Client) UserSpace(uid string) (json.RawMessage, error) {
	return c.get("/v6/user/space", url.Values{"uid": {uid}})
}

// UserFeedList 返回用户动态列表原始 data。
func (c *Client) UserFeedList(uid string, page int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("uid", uid)
	params.Set("page", strconv.Itoa(page))
	params.Set("showAnonymous", "0")
	params.Set("isIncludeTop", "1")
	params.Set("showDoing", "1")
	return c.get("/v6/user/feedList", params)
}

func (c *Client) get(path string, params url.Values) (json.RawMessage, error) {
	var lastErr error
	// token 被服务端判定无效时，重新签发一次再试。
	for attempt := 0; attempt < 2; attempt++ {
		data, err := c.do(path, params)
		if err == nil {
			return data, nil
		}
		lastErr = err
		var apiErr *Error
		if !errors.As(err, &apiErr) || !apiErr.TokenExpired() {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) do(path string, params url.Values) (json.RawMessage, error) {
	token, err := auth.Token(c.device, c.now().Unix())
	if err != nil {
		return nil, err
	}
	requestURL := c.baseURL + path
	if encoded := params.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("X-App-Id", auth.AppID)
	request.Header.Set("X-App-Token", token)
	request.Header.Set("X-App-Device", c.device)
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("X-Sdk-Int", sdkInt)
	request.Header.Set("X-Sdk-Locale", "zh-CN")
	request.Header.Set("X-App-Version", appVersion)
	request.Header.Set("X-App-Code", appCode)
	request.Header.Set("X-Api-Version", apiVersion)
	request.Header.Set("X-App-Channel", "coolapk")
	request.Header.Set("X-App-Mode", "universal")
	request.Header.Set("X-App-Supported", appCode)
	if c.cookie != "" {
		request.Header.Set("Cookie", c.cookie)
	}
	return c.execute(request, path)
}

// getWeb 请求网页版接口，用于移动端接口被风控时的只读兜底。
func (c *Client) getWeb(path string) (json.RawMessage, error) {
	request, err := http.NewRequest(http.MethodGet, c.webBaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	request.Header.Set("User-Agent", mobileUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.Header.Set("X-App-Id", auth.AppID)
	if c.cookie != "" {
		request.Header.Set("Cookie", c.cookie)
	}
	return c.execute(request, path)
}

func (c *Client) execute(request *http.Request, path string) (json.RawMessage, error) {
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("读取 %s 响应失败: %w", path, err)
	}
	return decodeData(body)
}

// decodeData 兼容裸 {"data":…} 与带 code/status/message 的信封结构。
func decodeData(body []byte) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("响应不是合法 JSON: %w", err)
	}
	apiErr := &Error{
		Status:        rawInt(fields["status"]),
		Code:          rawInt(fields["code"]),
		Message:       rawString(fields["message"]),
		MessageStatus: rawString(fields["messageStatus"]),
		MessageExtra:  rawString(fields["messageExtra"]),
	}
	switch {
	case apiErr.TokenExpired(), apiErr.Captcha():
		return nil, apiErr
	case apiErr.Code >= http.StatusBadRequest:
		return nil, apiErr
	case apiErr.Status != 0 && apiErr.Status != 1 && apiErr.Status != http.StatusOK:
		return nil, apiErr
	}
	data, ok := fields["data"]
	if !ok {
		return nil, fmt.Errorf("响应缺少 data 字段: %s", excerpt(body))
	}
	return data, nil
}

func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		if value, err := number.Int64(); err == nil {
			return int(value)
		}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if value, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
			return value
		}
	}
	return 0
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return strings.Trim(string(raw), `"`)
}

func excerpt(body []byte) string {
	const limit = 200
	text := strings.TrimSpace(string(body))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}
