package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/riba2534/feishu-cli/internal/auth"
	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

// resolveOptionalUserToken 解析显式指定的 user_access_token（可选）
// 仅检查 --user-access-token 参数和 FEISHU_USER_ACCESS_TOKEN 环境变量，
// 不自动从 token.json 加载，确保能用 App Token 的 API 默认使用 App Token（租户身份）
func resolveOptionalUserToken(cmd *cobra.Command) string {
	if flagToken, _ := cmd.Flags().GetString("user-access-token"); flagToken != "" {
		return flagToken
	}
	if envToken := os.Getenv("FEISHU_USER_ACCESS_TOKEN"); envToken != "" {
		return envToken
	}
	return ""
}

// resolveFlagUserToken 仅解析命令行显式传入的 user_access_token。
// 适用于默认应使用 App/Tenant Token，仅在用户明确指定时才切换到 User Token 的命令。
func resolveFlagUserToken(cmd *cobra.Command) string {
	flagToken, _ := cmd.Flags().GetString("user-access-token")
	return flagToken
}

// resolveOptionalUserTokenWithFallback 尝试完整优先级链解析 User Token（可选）
// 与 resolveOptionalUserToken 不同，会额外尝试从 token.json 和 config 中读取
// 找不到时返回空字符串（回退到 App Token），而非报错
// 适用于 msg/chat/doc export 等希望自动使用 User Token 的场景
func resolveOptionalUserTokenWithFallback(cmd *cobra.Command) string {
	flagToken, _ := cmd.Flags().GetString("user-access-token")
	cfg := config.Get()
	token, err := auth.ResolveUserAccessToken(flagToken, cfg.UserAccessToken, cfg.AppID, cfg.AppSecret, cfg.BaseURL)
	if err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[Debug] User Token 解析失败，回退到 App Token: %v\n", err)
		}
		return ""
	}
	if cfg.Debug {
		source := "token.json/config"
		if flagToken != "" {
			source = "--user-access-token 参数"
		} else if os.Getenv("FEISHU_USER_ACCESS_TOKEN") != "" {
			source = "FEISHU_USER_ACCESS_TOKEN 环境变量"
		}
		fmt.Fprintf(os.Stderr, "[Debug] 使用 User Access Token (来源: %s)\n", source)
	}
	return token
}

// resolveRequiredUserToken 解析 user_access_token（必需）
// 用于搜索等必须使用 User Access Token 的 API，解析失败时返回错误
func resolveRequiredUserToken(cmd *cobra.Command) (string, error) {
	flagToken, _ := cmd.Flags().GetString("user-access-token")
	cfg := config.Get()
	return auth.ResolveUserAccessToken(flagToken, cfg.UserAccessToken, cfg.AppID, cfg.AppSecret, cfg.BaseURL)
}

// resolveCurrentAuthedUserID returns the current logged-in user's ID for the requested type.
func resolveCurrentAuthedUserID(cmd *cobra.Command, userIDType string) (string, error) {
	token, err := resolveRequiredUserToken(cmd)
	if err != nil {
		return "", err
	}

	cfg := config.Get()
	cachePath, _ := auth.UserCachePath()
	cached, cacheErr := auth.LoadCurrentUserCache()
	switch {
	case cacheErr != nil:
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[Debug] [cache:%s] 读取当前登录用户缓存失败，回源 user_info: %v\n", cachePath, cacheErr)
		}
	case cached != nil && cached.MatchesToken(token):
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[Debug] [cache:%s] 命中当前登录用户缓存\n", cachePath)
		}
		return currentUserIDFromInfo(currentUserIDCacheToInfo(cached), userIDType)
	case cached != nil:
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[Debug] [cache:%s] 当前登录 token 已变化，忽略旧缓存并回源 user_info\n", cachePath)
		}
	default:
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[Debug] [cache:%s] 未命中当前登录用户缓存，回源 user_info\n", cachePath)
		}
	}

	info, err := client.GetCurrentUserInfo(token)
	if err != nil {
		return "", err
	}

	cache := &auth.CurrentUserCache{
		OpenID:           info.OpenID,
		UserID:           info.UserID,
		UnionID:          info.UnionID,
		Name:             info.Name,
		TokenFingerprint: auth.UserTokenFingerprint(token),
	}
	if err := auth.SaveCurrentUserCache(cache); err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[Debug] [cache:%s] 写入当前登录用户缓存失败: %v\n", cachePath, err)
		}
	} else if cfg.Debug {
		fmt.Fprintf(os.Stderr, "[Debug] [cache:%s] 已更新当前登录用户缓存\n", cachePath)
	}

	return currentUserIDFromInfo(info, userIDType)
}

func currentUserIDCacheToInfo(cache *auth.CurrentUserCache) *client.UserInfo {
	if cache == nil {
		return &client.UserInfo{}
	}

	return &client.UserInfo{
		OpenID:  cache.OpenID,
		UserID:  cache.UserID,
		UnionID: cache.UnionID,
		Name:    cache.Name,
	}
}

func currentUserIDFromInfo(info *client.UserInfo, userIDType string) (string, error) {
	if info == nil {
		return "", fmt.Errorf("当前登录用户信息为空")
	}

	switch userIDType {
	case "open_id":
		if info.OpenID != "" {
			return info.OpenID, nil
		}
	case "user_id":
		if info.UserID != "" {
			return info.UserID, nil
		}
	case "union_id":
		if info.UnionID != "" {
			return info.UnionID, nil
		}
	default:
		return "", fmt.Errorf("不支持的 user-id-type: %s", userIDType)
	}

	return "", fmt.Errorf("当前登录用户缺少 %s，无法自动推断当前登录用户身份", userIDType)
}

// validateEnum validates that value is one of the allowed values.
func validateEnum(value, fieldName string, allowedValues []string) error {
	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}
	return fmt.Errorf("不支持的%s %q，可选值: %s", fieldName, value, strings.Join(allowedValues, ", "))
}

// mustMarkFlagRequired 标记 flag 为必填，如果失败则 panic
// 用于 init() 函数中，确保配置错误在启动时被发现
func mustMarkFlagRequired(cmd *cobra.Command, flags ...string) {
	for _, flag := range flags {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			panic(fmt.Sprintf("标记必填 flag '%s' 失败: %v", flag, err))
		}
	}
}

// loadJSONInput 统一处理 --xxx 和 --xxx-file 两种 JSON 输入方式。
func loadJSONInput(inlineValue, filePath, inlineFlag, fileFlag, label string) (string, error) {
	if inlineValue != "" && filePath != "" {
		return "", fmt.Errorf("--%s 和 --%s 不能同时使用", inlineFlag, fileFlag)
	}

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("读取 %s 文件失败: %w", label, err)
		}
		inlineValue = string(data)
	}

	if strings.TrimSpace(inlineValue) == "" {
		return "", fmt.Errorf("请通过 --%s 或 --%s 提供%s", inlineFlag, fileFlag, label)
	}

	return inlineValue, nil
}

// printJSON 把 v 编码为带 2 空格缩进的 JSON 写到 stdout，禁用 HTML 转义。
// 用于命令一次性输出结构化结果（auth status、search docs -o json 等）。
func printJSON(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("JSON 序列化失败: %w", err)
	}
	fmt.Print(buf.String())
	return nil
}

// printJSONLine 把 v 编码为单行紧凑 JSON 写到 stdout，禁用 HTML 转义。
// 用于事件流（JSONL），例如 `auth login --json` 一次授权中依次输出
// device_authorization / authorization_success 两行事件。
func printJSONLine(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("JSON 序列化失败: %w", err)
	}
	// json.Encoder.Encode 已经追加了 \n，直接写出即可。
	fmt.Print(buf.String())
	return nil
}

// confirmAction 在执行危险操作前请求用户确认
// 返回 true 表示用户确认执行，false 表示取消
func confirmAction(prompt string) bool {
	fmt.Printf("%s (y/N): ", prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// validateOutputPath 验证输出路径是否安全
// 防止路径遍历攻击
func validateOutputPath(outputPath string, allowedDir string) error {
	// 清理路径
	cleanPath := filepath.Clean(outputPath)

	// 检查是否包含路径遍历
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("输出路径不能包含 '..'")
	}

	// 如果指定了允许的目录，验证路径在该目录下
	if allowedDir != "" {
		absOutput, err := filepath.Abs(cleanPath)
		if err != nil {
			return fmt.Errorf("无法解析输出路径: %w", err)
		}
		absAllowed, err := filepath.Abs(allowedDir)
		if err != nil {
			return fmt.Errorf("无法解析允许目录: %w", err)
		}
		if !strings.HasPrefix(absOutput, absAllowed) {
			return fmt.Errorf("输出路径必须在 %s 目录下", allowedDir)
		}
	}

	return nil
}

// unescapeSheetRange 处理 shell 转义的范围字符串
// 在某些 shell（如 zsh）中，! 字符会被自动转义为 \!
// 此函数将 \! 转换回 !
func unescapeSheetRange(rangeStr string) string {
	return strings.ReplaceAll(rangeStr, "\\!", "!")
}

// safeOutputPath 生成安全的输出路径
// 移除不安全的字符，防止路径遍历
func safeOutputPath(baseName string, ext string) string {
	// 移除路径分隔符和不安全字符
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, baseName)

	// 限制文件名长度
	if len(safeName) > 200 {
		safeName = safeName[:200]
	}

	if ext != "" && !strings.HasSuffix(safeName, ext) {
		safeName += ext
	}

	return safeName
}

// isValidToken 验证飞书 token 格式
// 飞书 token 通常由字母和数字组成，长度在 10-50 之间
func isValidToken(token string) bool {
	if len(token) < 5 || len(token) > 100 {
		return false
	}
	// 只允许字母、数字和部分特殊字符
	for _, r := range token {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// normalizePermMemberType normalizes member type aliases for the Drive permission API.
// The IM API uses underscore-separated identifiers (open_id, user_id, union_id, chat_id),
// while the Drive permission API uses concatenated identifiers (openid, userid, unionid, openchat).
// This function accepts both styles so users don't have to remember which API uses which format.
func normalizePermMemberType(memberType string) string {
	switch memberType {
	case "open_id":
		return "openid"
	case "user_id":
		return "userid"
	case "union_id":
		return "unionid"
	case "chat_id":
		return "openchat"
	default:
		return memberType
	}
}

// splitAndTrim 按逗号分割字符串并去除空白
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
