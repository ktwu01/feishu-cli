package cmd

import (
	"github.com/spf13/cobra"
)

// sheetCmd represents the sheet command group
var sheetCmd = &cobra.Command{
	Use:   "sheet",
	Short: "电子表格操作",
	Long:  "电子表格操作命令组，包括创建、读写、工作表管理等功能",
}

func init() {
	rootCmd.AddCommand(sheetCmd)
}
