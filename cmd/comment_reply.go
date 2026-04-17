package cmd

import (
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

var replyCmd = &cobra.Command{
	Use:   "reply",
	Short: "评论回复管理",
	Long: `评论回复管理命令，包括列出回复和删除回复。

子命令:
  list      列出评论的回复
  delete    删除评论回复

示例:
  # 列出评论回复
  feishu-cli comment reply list <file_token> <comment_id> --type docx

  # 删除评论回复
  feishu-cli comment reply delete <file_token> <comment_id> <reply_id> --type docx`,
}

var listReplyCmd = &cobra.Command{
	Use:   "list <file_token> <comment_id>",
	Short: "列出评论回复",
	Long: `列出指定评论的所有回复。

参数:
  file_token    文档 Token
  comment_id    评论 ID

示例:
  feishu-cli comment reply list doccnXXX 6916106822734578184 --type docx`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		fileToken := args[0]
		commentID := args[1]
		fileType, _ := cmd.Flags().GetString("type")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		output, _ := cmd.Flags().GetString("output")

		replies, _, _, err := client.ListCommentReplies(fileToken, commentID, fileType, pageSize, "")
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(replies)
		}

		if len(replies) == 0 {
			fmt.Println("暂无回复")
			return nil
		}

		fmt.Printf("共 %d 条回复:\n\n", len(replies))
		for i, r := range replies {
			fmt.Printf("[%d] 回复 ID: %s\n", i+1, r.ReplyID)
			if r.UserID != "" {
				fmt.Printf("    用户 ID: %s\n", r.UserID)
			}
			if r.Content != "" {
				fmt.Printf("    内容:     %s\n", r.Content)
			}
			if r.CreateTime != 0 {
				fmt.Printf("    创建时间: %d\n", r.CreateTime)
			}
			fmt.Println()
		}

		return nil
	},
}

var deleteReplyCmd = &cobra.Command{
	Use:   "delete <file_token> <comment_id> <reply_id>",
	Short: "删除评论回复",
	Long: `删除指定的评论回复。

参数:
  file_token    文档 Token
  comment_id    评论 ID
  reply_id      回复 ID

示例:
  feishu-cli comment reply delete doccnXXX 6916106822734578184 6916106822734594568 --type docx`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		fileToken := args[0]
		commentID := args[1]
		replyID := args[2]
		fileType, _ := cmd.Flags().GetString("type")

		if err := client.DeleteCommentReply(fileToken, commentID, replyID, fileType); err != nil {
			return err
		}

		fmt.Printf("回复删除成功！\n")
		fmt.Printf("  文档 Token: %s\n", fileToken)
		fmt.Printf("  评论 ID:    %s\n", commentID)
		fmt.Printf("  回复 ID:    %s\n", replyID)

		return nil
	},
}

func init() {
	commentCmd.AddCommand(replyCmd)

	replyCmd.AddCommand(listReplyCmd)
	listReplyCmd.Flags().String("type", "docx", "文件类型（doc/docx/sheet/bitable）")
	listReplyCmd.Flags().Int("page-size", 50, "每页数量")
	listReplyCmd.Flags().StringP("output", "o", "", "输出格式（json）")

	replyCmd.AddCommand(deleteReplyCmd)
	deleteReplyCmd.Flags().String("type", "docx", "文件类型（doc/docx/sheet/bitable）")
}
