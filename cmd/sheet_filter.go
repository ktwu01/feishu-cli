package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

// 筛选命令组
var sheetFilterCmd = &cobra.Command{
	Use:   "filter",
	Short: "筛选操作",
	Long:  "工作表筛选相关操作",
}

var sheetFilterCreateCmd = &cobra.Command{
	Use:   "create <spreadsheet_token> <sheet_id> <range>",
	Short: "创建筛选",
	Long: `在工作表中创建筛选。

示例:
  feishu-cli sheet filter create shtcnxxxxxx 0b12 "A1:C10"`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		sheetID := args[1]
		rangeStr := unescapeSheetRange(args[2])

		err := client.CreateFilter(client.Context(), spreadsheetToken, sheetID, rangeStr, nil)
		if err != nil {
			return err
		}

		fmt.Printf("筛选创建成功！范围: %s\n", rangeStr)
		return nil
	},
}

var sheetFilterGetCmd = &cobra.Command{
	Use:   "get <spreadsheet_token> <sheet_id>",
	Short: "获取筛选信息",
	Long:  "获取工作表的筛选信息",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		sheetID := args[1]
		outputFormat, _ := cmd.Flags().GetString("output")

		info, err := client.GetFilter(client.Context(), spreadsheetToken, sheetID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("筛选信息:\n")
			fmt.Printf("  范围: %s\n", info.Range)
			if len(info.FilteredRows) > 0 {
				fmt.Printf("  隐藏行: %v\n", info.FilteredRows)
			}
		}

		return nil
	},
}

var sheetFilterDeleteCmd = &cobra.Command{
	Use:   "delete <spreadsheet_token> <sheet_id>",
	Short: "删除筛选",
	Long:  "删除工作表的筛选",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		sheetID := args[1]

		err := client.DeleteFilter(client.Context(), spreadsheetToken, sheetID)
		if err != nil {
			return err
		}

		fmt.Println("筛选删除成功！")
		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetFilterCmd)

	sheetFilterCmd.AddCommand(sheetFilterCreateCmd)
	sheetFilterCmd.AddCommand(sheetFilterGetCmd)
	sheetFilterCmd.AddCommand(sheetFilterDeleteCmd)

	sheetFilterGetCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
