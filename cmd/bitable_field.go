package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

// 字段类型映射
var fieldTypeNames = map[int]string{
	1:    "多行文本",
	2:    "数字",
	3:    "单选",
	4:    "多选",
	5:    "日期",
	7:    "复选框",
	11:   "人员",
	15:   "超链接",
	18:   "单向关联",
	20:   "公式",
	21:   "双向关联",
	22:   "地理位置",
	23:   "群组",
	1001: "创建时间",
	1002: "修改时间",
	1003: "创建人",
	1004: "修改人",
	1005: "自动编号",
}

var bitableFieldsCmd = &cobra.Command{
	Use:   "fields <app_token> <table_id>",
	Short: "列出字段",
	Long:  "列出数据表的所有字段",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		fields, err := client.ListBitableFields(appToken, tableID, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(fields)
		}

		if len(fields) == 0 {
			fmt.Println("暂无字段")
			return nil
		}

		fmt.Printf("共 %d 个字段：\n", len(fields))
		for i, f := range fields {
			typeName := fieldTypeNames[f.Type]
			if typeName == "" {
				typeName = fmt.Sprintf("type=%d", f.Type)
			}
			primary := ""
			if f.IsPrimary {
				primary = " [主索引]"
			}
			fmt.Printf("  %d. %s (%s, ID: %s)%s\n", i+1, f.FieldName, typeName, f.FieldID, primary)
		}
		return nil
	},
}

var bitableCreateFieldCmd = &cobra.Command{
	Use:   "create-field <app_token> <table_id>",
	Short: "创建字段",
	Long: `创建数据表字段。

字段定义 JSON 格式:
  {"field_name": "名称", "type": 1}                              # 多行文本
  {"field_name": "金额", "type": 2}                              # 数字
  {"field_name": "状态", "type": 3, "property": {"options": [{"name": "进行中"}, {"name": "已完成"}]}}  # 单选

字段类型: 1=文本 2=数字 3=单选 4=多选 5=日期 7=复选框 11=人员 15=超链接 18=单向关联`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		fieldJSON, _ := cmd.Flags().GetString("field")
		fieldFile, _ := cmd.Flags().GetString("field-file")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		fieldJSON, err := loadJSONInput(fieldJSON, fieldFile, "field", "field-file", "字段定义 JSON")
		if err != nil {
			return err
		}

		var fieldDef map[string]any
		if err := json.Unmarshal([]byte(fieldJSON), &fieldDef); err != nil {
			return fmt.Errorf("解析字段定义 JSON 失败: %w", err)
		}

		field, err := client.CreateBitableField(appToken, tableID, fieldDef, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(field)
		}

		fmt.Printf("创建成功！\n")
		fmt.Printf("  Field ID: %s\n", field.FieldID)
		fmt.Printf("  名称: %s\n", field.FieldName)
		fmt.Printf("  类型: %d\n", field.Type)
		return nil
	},
}

var bitableUpdateFieldCmd = &cobra.Command{
	Use:   "update-field <app_token> <table_id> <field_id>",
	Short: "更新字段",
	Long: `更新数据表字段。

⚠️ 重要：更新单选（type=3）字段时，必须带上完整的 property（含 options），否则选项被清空。
⚠️ 重要：更新主索引列（is_primary=true）时，必须带 type 字段。

建议先用 fields 命令获取当前字段定义，修改后再更新。`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		fieldID := args[2]
		fieldJSON, _ := cmd.Flags().GetString("field")
		fieldFile, _ := cmd.Flags().GetString("field-file")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		fieldJSON, err := loadJSONInput(fieldJSON, fieldFile, "field", "field-file", "字段定义 JSON")
		if err != nil {
			return err
		}

		var fieldDef map[string]any
		if err := json.Unmarshal([]byte(fieldJSON), &fieldDef); err != nil {
			return fmt.Errorf("解析字段定义 JSON 失败: %w", err)
		}

		field, err := client.UpdateBitableField(appToken, tableID, fieldID, fieldDef, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(field)
		}

		fmt.Printf("更新成功！\n")
		fmt.Printf("  Field ID: %s\n", field.FieldID)
		fmt.Printf("  名称: %s\n", field.FieldName)
		return nil
	},
}

var bitableDeleteFieldCmd = &cobra.Command{
	Use:   "delete-field <app_token> <table_id> <field_id>",
	Short: "删除字段",
	Long:  "删除数据表的指定字段",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		fieldID := args[2]
		userToken := resolveOptionalUserToken(cmd)

		if err := client.DeleteBitableField(appToken, tableID, fieldID, userToken); err != nil {
			return err
		}

		fmt.Println("删除成功")
		return nil
	},
}

func init() {
	bitableCmd.AddCommand(bitableFieldsCmd)
	bitableCmd.AddCommand(bitableCreateFieldCmd)
	bitableCmd.AddCommand(bitableUpdateFieldCmd)
	bitableCmd.AddCommand(bitableDeleteFieldCmd)

	// fields
	bitableFieldsCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableFieldsCmd.Flags().String("user-access-token", "", "User Access Token（可选）")

	// create-field
	bitableCreateFieldCmd.Flags().String("field", "", "字段定义 JSON")
	bitableCreateFieldCmd.Flags().String("field-file", "", "字段定义 JSON 文件")
	bitableCreateFieldCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableCreateFieldCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	bitableCreateFieldCmd.MarkFlagsOneRequired("field", "field-file")
	bitableCreateFieldCmd.MarkFlagsMutuallyExclusive("field", "field-file")

	// update-field
	bitableUpdateFieldCmd.Flags().String("field", "", "字段定义 JSON")
	bitableUpdateFieldCmd.Flags().String("field-file", "", "字段定义 JSON 文件")
	bitableUpdateFieldCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableUpdateFieldCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	bitableUpdateFieldCmd.MarkFlagsOneRequired("field", "field-file")
	bitableUpdateFieldCmd.MarkFlagsMutuallyExclusive("field", "field-file")

	// delete-field
	bitableDeleteFieldCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
}
