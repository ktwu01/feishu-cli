package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/riba2534/feishu-cli/internal/auth"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "登录授权（获取 User Access Token）",
	Long: `通过 OAuth 2.0 完成用户授权，支持两种模式:

Authorization Code Flow（默认）:
  需要在飞书开放平台配置重定向 URL。
  · 本地桌面环境: 自动启动本地 HTTP 服务器并打开浏览器完成回调。
  · 远程 SSH 环境（自动检测或 --manual）: 打印授权 URL，手动复制回调 URL 粘贴到终端。
  · 非交互模式（--print-url）: 仅输出授权 URL JSON，配合 auth callback 两步完成。

Device Flow（--device，RFC 8628）:
  无需在飞书开放平台配置重定向 URL 白名单。
  终端显示用户码，用户在任意浏览器打开链接输入用户码完成授权，命令自动轮询等待结果。

Token 保存位置: ~/.feishu-cli/token.json

Authorization Code Flow 前置条件:
  在飞书开放平台 → 应用详情 → 安全设置 → 重定向 URL 中添加:
  http://127.0.0.1:9768/callback

示例:
  # 自动检测环境
  feishu-cli auth login

  # 强制手动模式（SSH 远程环境）
  feishu-cli auth login --manual

  # 指定端口
  feishu-cli auth login --port 8080

  # 指定 scope（建议带 offline_access 以获取 refresh_token）
  feishu-cli auth login --scopes "search:docs:read search:message offline_access"

  # 非交互模式（AI Agent 推荐）
  feishu-cli auth login --print-url
  # 然后用户在浏览器完成授权后执行:
  feishu-cli auth callback "<回调URL>" --state "<state>"

  # Device Flow（无需配置重定向 URL 白名单）
  feishu-cli auth login --method device`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		cfg := config.Get()
		scopes, _ := cmd.Flags().GetString("scopes")
		method, _ := cmd.Flags().GetString("method")

		switch method {
		case "device":
			return runDeviceFlow(cfg.AppID, cfg.AppSecret, cfg.BaseURL)
		case "code", "":
			// Authorization Code Flow，继续往下
		default:
			return fmt.Errorf("不支持的授权方式 %q，可选值: code, device", method)
		}

		// Authorization Code Flow
		port, _ := cmd.Flags().GetInt("port")
		manual, _ := cmd.Flags().GetBool("manual")
		noManual, _ := cmd.Flags().GetBool("no-manual")
		printURL, _ := cmd.Flags().GetBool("print-url")

		opts := auth.LoginOptions{
			Port:      port,
			Manual:    manual,
			NoManual:  noManual,
			AppID:     cfg.AppID,
			AppSecret: cfg.AppSecret,
			BaseURL:   cfg.BaseURL,
			Scopes:    scopes,
		}

		if printURL {
			result, err := auth.GenerateAuthURL(opts)
			if err != nil {
				return err
			}
			return printJSON(result)
		}

		token, err := auth.Login(opts)
		if err != nil {
			return err
		}

		printTokenSuccess(token)
		return nil
	},
}

// runDeviceFlow 执行 Device Flow 授权（RFC 8628）
func runDeviceFlow(appID, appSecret, baseURL string) error {
	deviceResp, err := auth.RequestDeviceAuthorization(appID, appSecret, baseURL, "")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "\n请在浏览器中完成以下操作:")
	fmt.Fprintln(os.Stderr, "─────────────────────────────────────────────")
	fmt.Fprintf(os.Stderr, "  1. 打开链接: %s\n", deviceResp.VerificationURI)
	fmt.Fprintf(os.Stderr, "  2. 输入用户码: %s\n", formatUserCode(deviceResp.UserCode))
	fmt.Fprintln(os.Stderr, "─────────────────────────────────────────────")
	if deviceResp.VerificationURIComplete != "" && deviceResp.VerificationURIComplete != deviceResp.VerificationURI {
		fmt.Fprintf(os.Stderr, "\n或直接访问完整链接（含用户码）:\n  %s\n", deviceResp.VerificationURIComplete)
	}
	fmt.Fprintf(os.Stderr, "\n等待授权（%d 秒后过期）...\n", deviceResp.ExpiresIn)

	openURL := deviceResp.VerificationURIComplete
	if openURL == "" {
		openURL = deviceResp.VerificationURI
	}
	_ = auth.TryOpenBrowser(openURL)

	lastLine := ""
	token, err := auth.PollDeviceToken(
		appID, appSecret, baseURL,
		deviceResp.DeviceCode, deviceResp.Interval, deviceResp.ExpiresIn,
		func(elapsed, total int) {
			line := fmt.Sprintf("\r  轮询中... 已等待 %ds / %ds", elapsed, total)
			if len(line) < len(lastLine) {
				line += strings.Repeat(" ", len(lastLine)-len(line))
			}
			lastLine = line
			fmt.Fprint(os.Stderr, line)
		},
	)
	if lastLine != "" {
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return err
	}

	if err := auth.SaveToken(token); err != nil {
		return err
	}

	printTokenSuccess(token)
	return nil
}

// formatUserCode 将用户码格式化为易读形式（8 位时加连字符，如 ABCD-EFGH）
func formatUserCode(code string) string {
	if strings.ContainsAny(code, "-_ ") {
		return code
	}
	if len(code) == 8 {
		return code[:4] + "-" + code[4:]
	}
	return code
}

// printTokenSuccess 打印授权成功信息
func printTokenSuccess(token *auth.TokenStore) {
	path, _ := auth.TokenPath()
	fmt.Fprintln(os.Stderr, "\n✓ 授权成功！")
	fmt.Fprintf(os.Stderr, "  Token 已保存到 %s\n", path)
	fmt.Fprintf(os.Stderr, "  Access Token 有效期至: %s\n", token.ExpiresAt.Format("2006-01-02 15:04:05"))
	if !token.RefreshExpiresAt.IsZero() {
		fmt.Fprintf(os.Stderr, "  Refresh Token 有效期至: %s\n", token.RefreshExpiresAt.Format("2006-01-02 15:04:05"))
	}
	if token.Scope != "" {
		fmt.Fprintf(os.Stderr, "  授权范围: %s\n", token.Scope)
	}
}

func init() {
	authCmd.AddCommand(authLoginCmd)

	authLoginCmd.Flags().Int("port", auth.DefaultPort, "本地回调服务器端口（Authorization Code Flow）")
	authLoginCmd.Flags().Bool("manual", false, "强制使用手动粘贴模式（Authorization Code Flow）")
	authLoginCmd.Flags().Bool("no-manual", false, "强制使用本地回调模式（Authorization Code Flow）")
	authLoginCmd.Flags().Bool("print-url", false, "仅输出授权 URL 和 state（Authorization Code Flow 非交互模式）")
	authLoginCmd.Flags().String("scopes", "", "请求的 OAuth scope（空格分隔，如 \"search:docs:read offline_access\"）")
	authLoginCmd.Flags().String("method", "code", "授权方式：code（Authorization Code Flow）或 device（Device Flow，无需配置重定向 URL）")
}
