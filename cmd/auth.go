package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "用户授权管理",
	Long: `管理 OAuth 2.0 用户授权，用于获取 User Access Token。

搜索、消息互动、审批任务等命令需要 User Access Token，通过 Device Flow（RFC 8628）完成授权，
无需在飞书开放平台配置任何重定向 URL 白名单。

示例:
  feishu-cli auth login                            # 登录（Device Flow）
  feishu-cli auth login --json                     # AI Agent 推荐：JSON 事件流输出
  feishu-cli auth check --scope "search:docs:read" # 检查当前 token 是否包含所需 scope
  feishu-cli auth status                           # 查看授权状态
  feishu-cli auth logout                           # 退出登录`,
}

func init() {
	rootCmd.AddCommand(authCmd)
}
