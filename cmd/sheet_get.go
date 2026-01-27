package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetGetCmd = &cobra.Command{
	Use:   "get <spreadsheet_token>",
	Short: "获取电子表格信息",
	Long:  "获取电子表格的基本信息",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		outputFormat, _ := cmd.Flags().GetString("output")

		info, err := client.GetSpreadsheet(client.Context(), spreadsheetToken)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("电子表格信息:\n")
			fmt.Printf("  Token: %s\n", info.SpreadsheetToken)
			fmt.Printf("  标题: %s\n", info.Title)
			if info.URL != "" {
				fmt.Printf("  URL: %s\n", info.URL)
			}
			if info.OwnerID != "" {
				fmt.Printf("  所有者: %s\n", info.OwnerID)
			}
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetGetCmd)

	sheetGetCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
