package cmd

import (
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var bitableViewsCmd = &cobra.Command{
	Use:   "views <app_token> <table_id>",
	Short: "列出视图",
	Long:  "列出数据表的视图，支持分页参数",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		output, _ := cmd.Flags().GetString("output")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		pageToken, _ := cmd.Flags().GetString("page-token")
		userToken := resolveOptionalUserToken(cmd)

		views, nextPageToken, err := client.ListBitableViews(appToken, tableID, pageSize, pageToken, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			result := map[string]any{
				"views": views,
			}
			if nextPageToken != "" {
				result["page_token"] = nextPageToken
				result["has_more"] = true
			}
			return printJSON(result)
		}

		if len(views) == 0 {
			fmt.Println("暂无视图")
			return nil
		}

		fmt.Printf("当前返回 %d 个视图", len(views))
		if nextPageToken != "" {
			fmt.Printf("（还有更多，page_token: %s）", nextPageToken)
		}
		fmt.Println("：")
		for i, v := range views {
			fmt.Printf("  %d. %s (类型: %s, ID: %s)\n", i+1, v.ViewName, v.ViewType, v.ViewID)
		}
		return nil
	},
}

var bitableCreateViewCmd = &cobra.Command{
	Use:   "create-view <app_token> <table_id>",
	Short: "创建视图",
	Long: `创建数据表视图。

视图类型:
  grid     表格视图（默认）
  kanban   看板视图
  gallery  画册视图
  gantt    甘特图视图
  form     表单视图`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		viewName, _ := cmd.Flags().GetString("name")
		viewType, _ := cmd.Flags().GetString("type")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		view, err := client.CreateBitableView(appToken, tableID, viewName, viewType, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(view)
		}

		fmt.Printf("创建成功！\n")
		fmt.Printf("  View ID: %s\n", view.ViewID)
		fmt.Printf("  名称: %s\n", view.ViewName)
		fmt.Printf("  类型: %s\n", view.ViewType)
		return nil
	},
}

var bitableDeleteViewCmd = &cobra.Command{
	Use:   "delete-view <app_token> <table_id> <view_id>",
	Short: "删除视图",
	Long:  "删除数据表的指定视图",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		viewID := args[2]
		userToken := resolveOptionalUserToken(cmd)

		if err := client.DeleteBitableView(appToken, tableID, viewID, userToken); err != nil {
			return err
		}

		fmt.Println("删除成功")
		return nil
	},
}

func init() {
	bitableCmd.AddCommand(bitableViewsCmd)
	bitableCmd.AddCommand(bitableCreateViewCmd)
	bitableCmd.AddCommand(bitableDeleteViewCmd)

	// views
	bitableViewsCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableViewsCmd.Flags().Int("page-size", 20, "每页视图数")
	bitableViewsCmd.Flags().String("page-token", "", "分页标记")
	bitableViewsCmd.Flags().String("user-access-token", "", "User Access Token（可选）")

	// create-view
	bitableCreateViewCmd.Flags().StringP("name", "n", "", "视图名称")
	bitableCreateViewCmd.Flags().StringP("type", "t", "grid", "视图类型: grid, kanban, gallery, gantt, form")
	bitableCreateViewCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableCreateViewCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	mustMarkFlagRequired(bitableCreateViewCmd, "name")

	// delete-view
	bitableDeleteViewCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
}
