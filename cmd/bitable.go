package cmd

import (
	"github.com/spf13/cobra"
)

var bitableCmd = &cobra.Command{
	Use:     "bitable",
	Aliases: []string{"base"},
	Short:   "多维表格操作",
	Long: `多维表格操作命令组，支持数据表、字段、记录、视图的增删改查。

子命令:
  create        创建多维表格
  get           获取多维表格信息
  tables        列出数据表
  create-table  创建数据表
  delete-table  删除数据表
  rename-table  重命名数据表
  fields        列出字段
  create-field  创建字段
  update-field  更新字段
  delete-field  删除字段
  records       搜索/列出记录
  get-record    获取单条记录
  add-record    创建记录
  add-records   批量创建记录
  update-record 更新记录
  update-records 批量更新记录
  delete-records 批量删除记录
  views         列出视图
  create-view   创建视图
  delete-view   删除视图

示例:
  # 创建多维表格
  feishu-cli bitable create --name "项目管理"

  # 列出数据表
  feishu-cli bitable tables <app_token>

  # 搜索记录
  feishu-cli bitable records <app_token> <table_id>

  # 创建记录
  feishu-cli bitable add-record <app_token> <table_id> --fields '{"名称":"测试","金额":100}'`,
}

func init() {
	rootCmd.AddCommand(bitableCmd)
}
