package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetAddSheetCmd = &cobra.Command{
	Use:   "add-sheet <spreadsheet_token>",
	Short: "添加工作表",
	Long:  "在电子表格中添加新的工作表",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		title, _ := cmd.Flags().GetString("title")
		index, _ := cmd.Flags().GetInt("index")
		outputFormat, _ := cmd.Flags().GetString("output")

		info, err := client.AddSheet(client.Context(), spreadsheetToken, title, index)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("添加成功！\n")
			fmt.Printf("  工作表 ID: %s\n", info.SheetID)
			fmt.Printf("  标题: %s\n", info.Title)
			fmt.Printf("  索引: %d\n", info.Index)
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetAddSheetCmd)

	sheetAddSheetCmd.Flags().StringP("title", "t", "新工作表", "工作表标题")
	sheetAddSheetCmd.Flags().Int("index", 0, "工作表位置索引（0 表示第一个）")
	sheetAddSheetCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
