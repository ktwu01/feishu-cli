package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetListSheetsCmd = &cobra.Command{
	Use:   "list-sheets <spreadsheet_token>",
	Short: "列出所有工作表",
	Long:  "列出电子表格中的所有工作表",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		outputFormat, _ := cmd.Flags().GetString("output")

		sheets, err := client.QuerySheets(client.Context(), spreadsheetToken)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(sheets, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("共 %d 个工作表:\n", len(sheets))
			for i, s := range sheets {
				hidden := ""
				if s.Hidden {
					hidden = " [隐藏]"
				}
				fmt.Printf("  %d. %s (ID: %s, 行: %d, 列: %d)%s\n",
					i+1, s.Title, s.SheetID, s.RowCount, s.ColCount, hidden)
			}
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetListSheetsCmd)

	sheetListSheetsCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
