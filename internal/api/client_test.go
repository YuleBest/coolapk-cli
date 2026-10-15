package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := New("device-test", "")
	client.baseURL = server.URL
	// 兜底路径也指向测试服务器，避免测试意外访问线上网页版。
	client.webBaseURL = server.URL
	client.now = func() time.Time { return time.Unix(1770000000, 0) }
	return client
}

func TestSearchSendsDocumentedParamsAndHeaders(t *testing.T) {
	var gotQuery url.Values
	var gotHeaders http.Header
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		gotQuery = request.URL.Query()
		gotHeaders = request.Header.Clone()
		writer.Write([]byte(`{"data":[]}`))
	})

	if _, err := client.Search(SearchFeed, "小米13", 3); err != nil {
		t.Fatalf("Search() 出错: %v", err)
	}

	want := map[string]string{
		"type":          "feed",
		"feedType":      "all",
		"sort":          "default",
		"searchValue":   "小米13",
		"page":          "3",
		"showAnonymous": "-1",
	}
	for key, value := range want {
		if got := gotQuery.Get(key); got != value {
			t.Errorf("查询参数 %s = %q, 期望 %q", key, got, value)
		}
	}
	for _, header := range []string{
		"X-App-Id", "X-App-Token", "X-App-Device", "X-Requested-With",
		"X-Sdk-Int", "X-Sdk-Locale", "X-App-Version", "X-App-Code",
		"X-Api-Version", "X-App-Channel", "X-App-Mode", "X-App-Supported", "User-Agent",
	} {
		if gotHeaders.Get(header) == "" {
			t.Errorf("缺少请求头 %s", header)
		}
	}
	if token := gotHeaders.Get("X-App-Token"); !strings.HasPrefix(token, "v3") {
		t.Errorf("X-App-Token = %q, 期望 v3 前缀", token)
	}
	if device := gotHeaders.Get("X-App-Device"); device != "device-test" {
		t.Errorf("X-App-Device = %q", device)
	}
	if cookie := gotHeaders.Get("Cookie"); cookie != "" {
		t.Errorf("未配置 Cookie 时不应发送 Cookie: %q", cookie)
	}
}

func TestSearchKindParams(t *testing.T) {
	cases := map[SearchKind]string{
		SearchUser: "user",
		SearchApk:  "apk",
	}
	for kind, wantType := range cases {
		client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
			if got := request.URL.Query().Get("type"); got != wantType {
				t.Errorf("type = %q, 期望 %q", got, wantType)
			}
			writer.Write([]byte(`{"data":[]}`))
		})
		if _, err := client.Search(kind, "酷安", 1); err != nil {
			t.Fatalf("Search(%s) 出错: %v", kind, err)
		}
	}
}

func TestSearchRejectsUnknownKind(t *testing.T) {
	client := New("device-test", "")
	if _, err := client.Search("bogus", "x", 1); err == nil {
		t.Fatal("未知搜索类型应当报错")
	}
}

func TestClientSendsCookieWhenConfigured(t *testing.T) {
	var got string
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		got = request.Header.Get("Cookie")
		writer.Write([]byte(`{"data":[]}`))
	})
	client.cookie = "uid=1; token=abc"
	if _, err := client.Search(SearchUser, "酷安", 1); err != nil {
		t.Fatalf("Search() 出错: %v", err)
	}
	if got != "uid=1; token=abc" {
		t.Fatalf("Cookie = %q", got)
	}
}

func TestDecodeEnvelope(t *testing.T) {
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"code":200,"message":"","status":1,"data":{"uid":"536381"}}`))
	})
	raw, err := client.UserSpace("536381")
	if err != nil {
		t.Fatalf("UserSpace() 出错: %v", err)
	}
	var user UserItem
	if err := json.Unmarshal(raw, &user); err != nil {
		t.Fatalf("解析 user 失败: %v", err)
	}
	if user.UID.String() != "536381" {
		t.Fatalf("uid = %q", user.UID.String())
	}
}

func TestDecodeBusinessError(t *testing.T) {
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"status":1001,"error":null,"message":"请求方式错误(1)"}`))
	})
	_, err := client.UserSpace("536381")
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("期望 *Error，实际 %v", err)
	}
	if apiErr.Status != 1001 || apiErr.Message != "请求方式错误(1)" {
		t.Fatalf("错误内容异常: %+v", apiErr)
	}
	if apiErr.Captcha() || apiErr.TokenExpired() {
		t.Fatalf("不应识别为风控或 token 失效: %+v", apiErr)
	}
}

func TestDecodeCaptchaError(t *testing.T) {
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"message":"当前访问需要验证码","messageStatus":"err_request_captcha_v2","code":403}`))
	})
	_, err := client.UserSpace("536381")
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("期望 *Error，实际 %v", err)
	}
	if !apiErr.Captcha() {
		t.Fatalf("应识别为风控: %+v", apiErr)
	}
}

func TestRetriesOnceOnTokenExpired(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if requests == 1 {
			writer.Write([]byte(`{"status":1004,"error":null,"message":"请求验证错误"}`))
			return
		}
		writer.Write([]byte(`{"data":[]}`))
	})
	if _, err := client.Search(SearchUser, "酷安", 1); err != nil {
		t.Fatalf("重试后应当成功: %v", err)
	}
	if requests != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", requests)
	}
}

func TestStopsAfterSingleRetry(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Write([]byte(`{"status":1004,"error":null,"message":"请求验证错误"}`))
	})
	_, err := client.Search(SearchUser, "酷安", 1)
	var apiErr *Error
	if !errors.As(err, &apiErr) || !apiErr.TokenExpired() {
		t.Fatalf("期望 token 失效错误，实际 %v", err)
	}
	if requests != 2 {
		t.Fatalf("请求次数 = %d, 期望 2", requests)
	}
}

func TestParseFeedsHandlesMixedTypes(t *testing.T) {
	raw := json.RawMessage(`[
		{"entityId":"74384253","likenum":"1","replynum":0,"isHeadline":1,
		 "dateline":1792074864,"username":"水墨青竹",
		 "replyRows":[{"id":123,"message":"<p>子评论</p>","likenum":"2"}]}
	]`)
	items, err := ParseFeeds(raw)
	if err != nil {
		t.Fatalf("ParseFeeds() 出错: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("条数 = %d", len(items))
	}
	item := items[0]
	if item.Identifier() != "74384253" || item.LikeNum.String() != "1" || item.ReplyNum.String() != "0" {
		t.Fatalf("字段解析异常: %+v", item)
	}
	if seconds, err := item.Dateline.Int64(); err != nil || seconds != 1792074864 {
		t.Fatalf("dateline 解析异常: %v %v", seconds, err)
	}
	if len(item.ReplyRows) != 1 || item.ReplyRows[0].Message != "<p>子评论</p>" {
		t.Fatalf("子评论解析异常: %+v", item.ReplyRows)
	}
}

func TestParseFeedNull(t *testing.T) {
	if _, err := ParseFeed(json.RawMessage("null")); err == nil {
		t.Fatal("空 data 应当报错")
	}
	items, err := ParseFeeds(json.RawMessage("null"))
	if err != nil || len(items) != 0 {
		t.Fatalf("空列表应返回空结果: %v %v", items, err)
	}
}

func TestDecodeDataRequiresDataField(t *testing.T) {
	if _, err := decodeData([]byte(`{"code":200}`)); err == nil {
		t.Fatal("缺少 data 字段应当报错")
	}
}

func TestRawIntAcceptsStrings(t *testing.T) {
	if got := rawInt(json.RawMessage(`"403"`)); got != 403 {
		t.Fatalf("rawInt = %d", got)
	}
	if got := rawInt(json.RawMessage(`null`)); got != 0 {
		t.Fatalf("rawInt(null) = %d", got)
	}
}

func TestFeedDetailFallsBackToWeb(t *testing.T) {
	apiRequests, webRequests := 0, 0
	var webHeaders http.Header
	var webPath string

	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		apiRequests++
		writer.Write([]byte(`{"message":"当前访问需要验证码","messageStatus":"err_request_captcha_v2","code":403}`))
	}))
	defer apiServer.Close()
	webServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		webRequests++
		webHeaders = request.Header.Clone()
		webPath = request.URL.Path
		writer.Write([]byte(`{"status":0,"data":{"entityId":"74360444","username":"看有个黑洞","message":"正文内容"}}`))
	}))
	defer webServer.Close()

	client := New("device-test", "uid=1; token=abc")
	client.baseURL = apiServer.URL
	client.webBaseURL = webServer.URL
	client.now = func() time.Time { return time.Unix(1770000000, 0) }

	raw, err := client.FeedDetail("74360444")
	if err != nil {
		t.Fatalf("应通过网页版兜底拿到正文: %v", err)
	}
	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if feed.Username != "看有个黑洞" || feed.Message != "正文内容" {
		t.Fatalf("正文内容异常: %+v", feed)
	}
	if apiRequests != 1 || webRequests != 1 {
		t.Fatalf("请求次数 api=%d web=%d, 期望各 1 次", apiRequests, webRequests)
	}
	if webPath != "/feed/74360444" {
		t.Fatalf("网页版路径 = %q", webPath)
	}
	if got := webHeaders.Get("X-Requested-With"); got != "XMLHttpRequest" {
		t.Fatalf("网页版缺少 X-Requested-With: %q", got)
	}
	if got := webHeaders.Get("X-App-Id"); got != "com.coolapk.market" {
		t.Fatalf("网页版 X-App-Id = %q", got)
	}
	if got := webHeaders.Get("Cookie"); got != "uid=1; token=abc" {
		t.Fatalf("网页版 Cookie = %q", got)
	}
	if got := webHeaders.Get("X-App-Token"); got != "" {
		t.Fatalf("网页版兜底不应带 X-App-Token: %q", got)
	}
}

func TestFeedDetailSkipsWebFallbackOnSuccess(t *testing.T) {
	webRequests := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"data":{"entityId":"1","message":"正文"}}`))
	}))
	defer apiServer.Close()
	webServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		webRequests++
		writer.Write([]byte(`{"status":0,"data":{}}`))
	}))
	defer webServer.Close()

	client := New("device-test", "")
	client.baseURL = apiServer.URL
	client.webBaseURL = webServer.URL
	client.now = func() time.Time { return time.Unix(1770000000, 0) }

	if _, err := client.FeedDetail("1"); err != nil {
		t.Fatalf("FeedDetail() 出错: %v", err)
	}
	if webRequests != 0 {
		t.Fatalf("移动端接口正常时不应请求网页版，实际 %d 次", webRequests)
	}
}

func TestFeedDetailReportsBothFailures(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"message":"当前访问需要验证码","messageStatus":"err_request_captcha_v2","code":403}`))
	}))
	defer apiServer.Close()
	webServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"status":400,"message":"你查看的内容不存在或已被删除"}`))
	}))
	defer webServer.Close()

	client := New("device-test", "")
	client.baseURL = apiServer.URL
	client.webBaseURL = webServer.URL
	client.now = func() time.Time { return time.Unix(1770000000, 0) }

	_, err := client.FeedDetail("1")
	if err == nil {
		t.Fatal("两条路径都失败时应当报错")
	}
	for _, want := range []string{"网页版兜底也失败", "不存在或已被删除"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息缺少 %q: %v", want, err)
		}
	}
}
