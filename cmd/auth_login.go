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
	Long: `通过 OAuth 2.0 Device Flow（RFC 8628）完成用户授权。

无需在飞书开放平台配置任何重定向 URL 白名单。终端显示用户码和验证链接，
用户在任意浏览器打开链接输入用户码完成授权，命令自动轮询等待结果。

Token 保存位置: ~/.feishu-cli/token.json

示例:
  # 标准登录（人类用户，本地或 SSH 远程均可）
  feishu-cli auth login

  # JSON 输出模式（AI Agent 推荐：run_in_background + 读 stdout 事件流）
  feishu-cli auth login --json

  # 两步模式第一步：只请求 device_code 并输出，不启动轮询
  feishu-cli auth login --no-wait --json

  # 两步模式第二步：用已有的 device_code 继续轮询
  feishu-cli auth login --device-code <device_code> --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		cfg := config.Get()
		jsonOutput, _ := cmd.Flags().GetBool("json")
		noWait, _ := cmd.Flags().GetBool("no-wait")
		deviceCode, _ := cmd.Flags().GetString("device-code")

		return runDeviceFlow(cfg, jsonOutput, deviceCode, noWait)
	},
}

// runDeviceFlow 执行 Device Flow 授权（RFC 8628）。
//
// 根据 deviceCode / noWait 参数触发四种行为：默认阻塞轮询；--json 事件流；
// --no-wait 立即返回 device_code 不轮询；--device-code 复用已有 device_code 继续轮询。
func runDeviceFlow(cfg *config.Config, jsonOutput bool, deviceCode string, noWait bool) error {
	appID := cfg.AppID
	appSecret := cfg.AppSecret
	baseURL := cfg.BaseURL

	var deviceResp *auth.DeviceAuthResponse

	if deviceCode == "" {
		// 步骤一：请求 device_authorization。
		// scope 传空，device_flow.go 会自动注入 offline_access；飞书 token 端点
		// 实际返回的 scope 由应用在开放平台预配置的权限决定。
		resp, err := auth.RequestDeviceAuthorization(appID, appSecret, baseURL, "")
		if err != nil {
			return err
		}
		deviceResp = resp

		if jsonOutput || noWait {
			event := map[string]any{
				"event":                     "device_authorization",
				"verification_uri":          deviceResp.VerificationURI,
				"verification_uri_complete": deviceResp.VerificationURIComplete,
				"user_code":                 deviceResp.UserCode,
				"device_code":               deviceResp.DeviceCode,
				"expires_in":                deviceResp.ExpiresIn,
				"interval":                  deviceResp.Interval,
			}
			if err := printJSONLine(event); err != nil {
				return err
			}
		} else {
			printDeviceAuthHuman(deviceResp)
			_ = auth.TryOpenBrowser(bestVerificationURL(deviceResp))
		}

		if noWait {
			return nil
		}
	} else {
		// 两步模式第二步：复用调用方提供的 device_code，用保守的默认轮询参数。
		// 与官方 lark-cli 的 authLoginPollDeviceCode 行为对齐。
		deviceResp = &auth.DeviceAuthResponse{
			DeviceCode: deviceCode,
			Interval:   5,
			ExpiresIn:  180,
		}
	}

	// 步骤二：轮询 token 端点。
	onTick := func(elapsed, total int) {
		if jsonOutput {
			return
		}
		fmt.Fprintf(os.Stderr, "\r  轮询中... 已等待 %ds / %ds", elapsed, total)
	}
	token, err := auth.PollDeviceToken(
		appID, appSecret, baseURL,
		deviceResp.DeviceCode, deviceResp.Interval, deviceResp.ExpiresIn,
		onTick,
	)
	if !jsonOutput {
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return err
	}

	if err := auth.SaveToken(token); err != nil {
		return err
	}

	if jsonOutput {
		event := map[string]any{
			"event":      "authorization_success",
			"expires_at": token.ExpiresAt.Format("2006-01-02T15:04:05+08:00"),
			"scope":      token.Scope,
		}
		if !token.RefreshExpiresAt.IsZero() {
			event["refresh_expires_at"] = token.RefreshExpiresAt.Format("2006-01-02T15:04:05+08:00")
		}
		return printJSONLine(event)
	}

	printTokenSuccess(token)
	return nil
}

// bestVerificationURL 优先返回 VerificationURIComplete，否则回退到 VerificationURI。
func bestVerificationURL(resp *auth.DeviceAuthResponse) string {
	if resp.VerificationURIComplete != "" {
		return resp.VerificationURIComplete
	}
	return resp.VerificationURI
}

// printDeviceAuthHuman 把设备授权信息按人类友好格式打印到 stderr。
func printDeviceAuthHuman(resp *auth.DeviceAuthResponse) {
	fmt.Fprintln(os.Stderr, "\n请在浏览器中完成以下操作:")
	fmt.Fprintln(os.Stderr, "─────────────────────────────────────────────")
	fmt.Fprintf(os.Stderr, "  1. 打开链接: %s\n", resp.VerificationURI)
	fmt.Fprintf(os.Stderr, "  2. 输入用户码: %s\n", formatUserCode(resp.UserCode))
	fmt.Fprintln(os.Stderr, "─────────────────────────────────────────────")
	if resp.VerificationURIComplete != "" && resp.VerificationURIComplete != resp.VerificationURI {
		fmt.Fprintf(os.Stderr, "\n或直接访问完整链接（含用户码）:\n  %s\n", resp.VerificationURIComplete)
	}
	fmt.Fprintf(os.Stderr, "\n等待授权（%d 秒后过期）...\n", resp.ExpiresIn)
}

// formatUserCode 将 8 位无分隔符的用户码格式化为 ABCD-EFGH。
func formatUserCode(code string) string {
	if strings.ContainsAny(code, "-_ ") {
		return code
	}
	if len(code) == 8 {
		return code[:4] + "-" + code[4:]
	}
	return code
}

// printTokenSuccess 打印授权成功信息到 stderr（人类模式）。
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

	authLoginCmd.Flags().Bool("json", false, "JSON 输出模式（AI Agent 友好，事件流写入 stdout）")
	authLoginCmd.Flags().Bool("no-wait", false, "只请求 device_code 并立即输出，不启动轮询（两步模式第一步）")
	authLoginCmd.Flags().String("device-code", "", "用已有的 device_code 继续轮询（两步模式第二步）")
}
