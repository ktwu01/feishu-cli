package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetMetaCmd = &cobra.Command{
	Use:   "meta <spreadsheet_token>",
	Short: "获取表格元信息",
	Long: `获取电子表格的详细元信息，包括工作表列表、权限等。

示例:
  feishu-cli sheet meta shtcnxxxxxx
  feishu-cli sheet meta shtcnxxxxxx --ext-fields "protectedRange,mergedCell"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		extFields, _ := cmd.Flags().GetString("ext-fields")
		outputFormat, _ := cmd.Flags().GetString("output")

		meta, err := client.GetSpreadsheetMeta(client.Context(), spreadsheetToken, extFields)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(meta, "", "  ")
			fmt.Println(string(output))
		} else {
			// 解析并显示主要信息
			if properties, ok := meta["properties"].(map[string]interface{}); ok {
				fmt.Printf("电子表格属性:\n")
				if title, ok := properties["title"].(string); ok {
					fmt.Printf("  标题: %s\n", title)
				}
				if ownerUser, ok := properties["ownerUser"].(float64); ok {
					fmt.Printf("  所有者 ID: %.0f\n", ownerUser)
				}
				if sheetCount, ok := properties["sheetCount"].(float64); ok {
					fmt.Printf("  工作表数量: %.0f\n", sheetCount)
				}
				if revision, ok := properties["revision"].(float64); ok {
					fmt.Printf("  版本: %.0f\n", revision)
				}
			}

			if sheets, ok := meta["sheets"].([]interface{}); ok {
				fmt.Printf("\n工作表列表:\n")
				for i, s := range sheets {
					if sheet, ok := s.(map[string]interface{}); ok {
						title := sheet["title"]
						sheetID := sheet["sheetId"]
						fmt.Printf("  %d. %v (ID: %v)\n", i+1, title, sheetID)
					}
				}
			}
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetMetaCmd)

	sheetMetaCmd.Flags().String("ext-fields", "", "扩展字段（逗号分隔）: protectedRange, mergedCell")
	sheetMetaCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
