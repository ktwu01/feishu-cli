package client

import (
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

// UserTokenOption 当 userAccessToken 非空时返回请求选项，否则返回 nil（回退到 tenant token）
func UserTokenOption(userAccessToken string) []larkcore.RequestOptionFunc {
	if userAccessToken != "" {
		return []larkcore.RequestOptionFunc{larkcore.WithUserAccessToken(userAccessToken)}
	}
	return nil
}

// StringVal 安全解引用字符串指针，nil 返回空字符串
func StringVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// BoolVal 安全解引用布尔指针，nil 返回 false
func BoolVal(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

// IntVal 安全解引用 int 指针，nil 返回 0
func IntVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// Int64Val 安全解引用 int64 指针，nil 返回 0
func Int64Val(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// IsRateLimitError 判断错误是否为频率限制错误
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "99991400") ||
		strings.Contains(msg, "frequency limit") ||
		strings.Contains(msg, "rate limit")
}

// IsRetryableError 判断错误是否可重试（服务端临时错误）
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "internal error") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "frequency limit")
}

// IsPermanentError 判断错误是否为永久性错误（不应重试）
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Parse error") ||
		strings.Contains(msg, "Invalid request parameter")
}
