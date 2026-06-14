package cmd

import (
	"fmt"
	"sort"
	"sync"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

// zeroSizeTypes 是无字节大小、对云空间容量贡献约为 0 的文档类型。
// 按大小排查存储占用时跳过它们，只统计上传素材 (type=file)。
var zeroSizeTypes = map[string]bool{
	"docx": true, "doc": true, "sheet": true, "bitable": true,
	"mindnote": true, "slides": true, "shortcut": true, "folder": true,
}

// biggestEntry 是一个已测大小的文件条目。
type biggestEntry struct {
	Size  int64  `json:"size"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Path  string `json:"path"`
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

var fileBiggestCmd = &cobra.Command{
	Use:   "biggest [folder_token]",
	Short: "递归扫描并按大小排序列出最大的文件（只读）",
	Long: `递归遍历云空间文件夹，测量每个上传文件的字节大小，按从大到小排序输出。

为什么需要这个命令:
  飞书 Drive 的 list 接口不返回文件大小，付费版网页端才提供按大小排序。
  本命令用合法的只读方式还原该能力：对每个 type=file 的素材取临时下载链接，
  再发 HTTP HEAD 读取 Content-Length，全程不下载文件内容、不做任何修改。

  docx/sheet/bitable 等无字节大小且几乎不占容量，默认跳过；占用容量的主要是
  上传的附件、图片、录制等普通文件，正是本命令排查的对象。

参数:
  folder_token    起始文件夹 Token（不指定则从「我的空间」根目录开始）

选项:
  --top N         输出前 N 个最大文件（默认 30）
  --concurrency   并发测量数（默认 8）
  --output json   以 JSON 输出完整结果

注意:
  - 只读操作，绝不删除/移动/清空任何文件。
  - tenant token 只能看到应用自己拥有的文件；要扫描你的个人空间，
    请先 feishu-cli auth login 完成用户授权（或用 --user-token）。

示例:
  feishu-cli file biggest --top 50
  feishu-cli file biggest fldcnXXXX --top 20 --output json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		var folderToken string
		if len(args) > 0 {
			folderToken = args[0]
		}
		top, _ := cmd.Flags().GetInt("top")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserTokenWithFallback(cmd)

		if concurrency < 1 {
			concurrency = 1
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "正在递归遍历文件夹 %s ...\n", folderOrRoot(folderToken))
		entries, err := client.ListFolderRecursive(folderToken, userToken)
		if err != nil {
			return fmt.Errorf("递归列出文件失败: %w", err)
		}

		// 仅保留有字节大小的上传文件。
		var targets []client.DriveRemoteEntry
		for _, e := range entries {
			if !zeroSizeTypes[e.Type] {
				targets = append(targets, e)
			}
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "共 %d 个条目，其中 %d 个上传文件需要测量大小\n", len(entries), len(targets))

		results := measureSizes(targets, userToken, concurrency, cmd)

		sort.Slice(results, func(i, j int) bool {
			return results[i].Size > results[j].Size
		})

		if output == "json" {
			return printJSON(results)
		}

		if len(results) == 0 {
			fmt.Println("未找到任何上传文件。可能是该空间仅含文档，或当前 token 无访问权限（试试 --user-token / auth login）。")
			return nil
		}

		limit := top
		if limit <= 0 || limit > len(results) {
			limit = len(results)
		}
		fmt.Printf("\n%9s  %-8s  %s\n", "大小", "类型", "名称 / 路径")
		fmt.Println("------------------------------------------------------------------------------")
		for _, r := range results[:limit] {
			note := ""
			if r.Error != "" {
				note = "  (" + r.Error + ")"
			}
			fmt.Printf("%9s  %-8s  %s%s\n", humanSize(r.Size), r.Type, r.Path, note)
		}
		return nil
	},
}

// measureSizes 并发测量每个文件的字节大小，限并发 workers。
func measureSizes(targets []client.DriveRemoteEntry, userToken string, workers int, cmd *cobra.Command) []biggestEntry {
	results := make([]biggestEntry, len(targets))
	var wg sync.WaitGroup
	var done int
	var mu sync.Mutex
	sem := make(chan struct{}, workers)

	for i, t := range targets {
		i, t := i, t
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			entry := biggestEntry{
				Name:  lastPathSegment(t.RelPath),
				Type:  t.Type,
				Path:  t.RelPath,
				Token: t.FileToken,
			}
			size, err := client.GetMediaSize(t.FileToken, client.DownloadMediaOptions{UserAccessToken: userToken})
			if err != nil {
				entry.Error = err.Error()
			} else {
				entry.Size = size
			}
			results[i] = entry

			mu.Lock()
			done++
			if done%25 == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "已测量 %d/%d ...\n", done, len(targets))
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

func folderOrRoot(token string) string {
	if token == "" {
		return "ROOT（我的空间）"
	}
	return token
}

func lastPathSegment(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// humanSize 把字节数格式化为人类可读形式。
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTP"[exp])
}

func init() {
	fileBiggestCmd.Flags().Int("top", 30, "输出前 N 个最大文件")
	fileBiggestCmd.Flags().Int("concurrency", 8, "并发测量数")
	fileBiggestCmd.Flags().String("output", "", "输出格式 (json)")
	fileCmd.AddCommand(fileBiggestCmd)
}
