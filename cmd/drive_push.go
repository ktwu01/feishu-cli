package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

var drivePushCmd = &cobra.Command{
	Use:   "push",
	Short: "把本地目录镜像到云盘文件夹（本地 → Drive，单向 file-level 镜像）",
	Long: `递归遍历 --local-dir 下的所有 regular file，上传到 --folder-token 下的对应路径。
本地目录会通过 /open-apis/drive/v1/files/create_folder 在远端按需创建以镜像目录结构。

可选 --delete-remote --yes 同时清理远端 type=file 但本地不存在的文件（高危，必须双确认）。
docx/sheet/bitable/mindnote/slides/shortcut 等在线文档不会被作为孤儿删除。
失败时不会触发删除阶段，避免「半同步」状态。

必填:
  --folder-token   云盘根文件夹 token
  --local-dir      本地根目录（必须在 cwd 子树内）

可选:
  --if-exists       skip（默认）/ overwrite：远端同路径已存在时如何处理
                    overwrite 走 upload_all 的 file_token 字段；如果租户尚未灰度该字段，覆盖会失败
  --delete-remote   清理远端不存在于本地的 file（高危）
  --yes             与 --delete-remote 配套，确认删除
  --output / -o     输出格式（json）
  --user-access-token  覆盖登录态

权限:
  - User Access Token 或 Tenant Token
  - drive:drive.metadata:readonly
  - drive:file:upload
  - 删除时需 drive:file:delete

示例:
  feishu-cli drive push --folder-token fldxxx --local-dir ./mirror
  feishu-cli drive push --folder-token fldxxx --local-dir ./mirror --if-exists overwrite
  feishu-cli drive push --folder-token fldxxx --local-dir ./mirror --delete-remote --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		folderToken, _ := cmd.Flags().GetString("folder-token")
		localDir, _ := cmd.Flags().GetString("local-dir")
		ifExists, _ := cmd.Flags().GetString("if-exists")
		deleteRemote, _ := cmd.Flags().GetBool("delete-remote")
		yes, _ := cmd.Flags().GetBool("yes")
		output, _ := cmd.Flags().GetString("output")

		if folderToken == "" {
			return fmt.Errorf("--folder-token 必填")
		}
		if localDir == "" {
			return fmt.Errorf("--local-dir 必填")
		}
		if ifExists == "" {
			ifExists = driveMirrorIfExistsSkip
		}
		if ifExists != driveMirrorIfExistsOverwrite && ifExists != driveMirrorIfExistsSkip {
			return fmt.Errorf("--if-exists 只能是 overwrite 或 skip")
		}
		if deleteRemote && !yes {
			return fmt.Errorf("--delete-remote 是高危操作，必须同时加 --yes 才执行")
		}

		safeRoot, _, err := resolveSafeLocalDir(localDir)
		if err != nil {
			return err
		}

		userToken := resolveOptionalUserTokenWithFallback(cmd)

		fmt.Fprintf(cmd.ErrOrStderr(), "扫描本地: %s\n", safeRoot)
		localFiles, err := walkLocalRegularFiles(safeRoot)
		if err != nil {
			return err
		}
		localDirs, err := walkLocalDirs(safeRoot)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "列举云盘文件夹: %s\n", folderToken)
		entries, err := client.ListFolderRecursive(folderToken, userToken)
		if err != nil {
			return err
		}
		remoteFiles := remoteFilesOnly(entries) // rel → file_token
		remoteFolders := make(map[string]string, len(entries))
		for rel, e := range entries {
			if e.Type == "folder" {
				remoteFolders[rel] = e.FileToken
			}
		}

		// folderCache: relDir → folder_token，root 关联到 folderToken
		folderCache := map[string]string{"": folderToken}
		for rel, tok := range remoteFolders {
			folderCache[rel] = tok
		}

		type item struct {
			RelPath   string `json:"rel_path"`
			FileToken string `json:"file_token,omitempty"`
			Action    string `json:"action"` // uploaded / overwritten / skipped / failed / folder_created / deleted_remote / delete_failed
			Error     string `json:"error,omitempty"`
		}
		var items []item
		var uploaded, skipped, failed, deletedRemote int
		uploadFailed := false

		// 先按本地目录创建远端文件夹（保证空目录也被镜像）
		sort.Strings(localDirs)
		for _, relDir := range localDirs {
			if _, ok := folderCache[relDir]; ok {
				continue
			}
			tok, fErr := ensureRemoteFolder(folderToken, relDir, folderCache, userToken)
			if fErr != nil {
				items = append(items, item{RelPath: relDir, Action: "failed", Error: fErr.Error()})
				failed++
				uploadFailed = true
				continue
			}
			items = append(items, item{RelPath: relDir, FileToken: tok, Action: "folder_created"})
		}

		// 再上传文件
		localPaths := make([]string, 0, len(localFiles))
		for rel := range localFiles {
			localPaths = append(localPaths, rel)
		}
		sort.Strings(localPaths)

		for _, rel := range localPaths {
			abs := localFiles[rel]

			// 远端同路径已有 file
			if existingToken, has := remoteFiles[rel]; has {
				if ifExists == driveMirrorIfExistsSkip {
					items = append(items, item{RelPath: rel, FileToken: existingToken, Action: "skipped"})
					skipped++
					continue
				}
				// overwrite：先删旧文件，再上传新文件到同 parent
				// 注意：upload_all 没有 update 语义，简单做法是 delete + upload
				parent := pushParentRel(rel)
				parentToken, ensureErr := ensureRemoteFolder(folderToken, parent, folderCache, userToken)
				if ensureErr != nil {
					items = append(items, item{RelPath: rel, FileToken: existingToken, Action: "failed", Error: ensureErr.Error()})
					failed++
					uploadFailed = true
					continue
				}
				if delErr := client.DeleteDriveFileByToken(existingToken, userToken); delErr != nil {
					items = append(items, item{RelPath: rel, FileToken: existingToken, Action: "failed", Error: delErr.Error()})
					failed++
					uploadFailed = true
					continue
				}
				newToken, upErr := client.UploadFileWithToken(abs, parentToken, filepath.Base(abs), userToken)
				if upErr != nil {
					items = append(items, item{RelPath: rel, Action: "failed", Error: upErr.Error()})
					failed++
					uploadFailed = true
					continue
				}
				items = append(items, item{RelPath: rel, FileToken: newToken, Action: "overwritten"})
				uploaded++
				continue
			}

			// 新文件
			parent := pushParentRel(rel)
			parentToken, ensureErr := ensureRemoteFolder(folderToken, parent, folderCache, userToken)
			if ensureErr != nil {
				items = append(items, item{RelPath: rel, Action: "failed", Error: ensureErr.Error()})
				failed++
				uploadFailed = true
				continue
			}
			newToken, upErr := client.UploadFileWithToken(abs, parentToken, filepath.Base(abs), userToken)
			if upErr != nil {
				items = append(items, item{RelPath: rel, Action: "failed", Error: upErr.Error()})
				failed++
				uploadFailed = true
				continue
			}
			items = append(items, item{RelPath: rel, FileToken: newToken, Action: "uploaded"})
			uploaded++
		}

		// --delete-remote 在上传阶段无失败时才执行
		if deleteRemote && !uploadFailed {
			remotePaths := make([]string, 0, len(remoteFiles))
			for rel := range remoteFiles {
				remotePaths = append(remotePaths, rel)
			}
			sort.Strings(remotePaths)
			for _, rel := range remotePaths {
				if _, has := localFiles[rel]; has {
					continue
				}
				token := remoteFiles[rel]
				if delErr := client.DeleteDriveFileByToken(token, userToken); delErr != nil {
					items = append(items, item{RelPath: rel, FileToken: token, Action: "delete_failed", Error: delErr.Error()})
					failed++
					continue
				}
				items = append(items, item{RelPath: rel, FileToken: token, Action: "deleted_remote"})
				deletedRemote++
			}
		} else if deleteRemote && uploadFailed {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"⚠ 跳过 --delete-remote：上面有 %d 个上传失败，避免半同步状态。修复后重跑。\n", failed)
		}

		summary := map[string]any{
			"uploaded":       uploaded,
			"skipped":        skipped,
			"failed":         failed,
			"deleted_remote": deletedRemote,
		}
		payload := map[string]any{"summary": summary, "items": items}

		if output == "json" {
			if err := printJSON(payload); err != nil {
				return err
			}
		} else {
			fmt.Printf("上传: %d  跳过: %d  删除远端: %d  失败: %d\n",
				uploaded, skipped, deletedRemote, failed)
			for _, it := range items {
				if it.Action == "failed" || it.Action == "delete_failed" {
					fmt.Printf("  ⚠ %-15s %s -- %s\n", it.Action, it.RelPath, it.Error)
				}
			}
		}

		if failed > 0 {
			return fmt.Errorf("有 %d 项失败，处于部分同步状态；修复后重跑", failed)
		}
		return nil
	},
}

// pushParentRel 取 rel_path（用 / 分隔）的父目录 rel_path。"" 表示根。
func pushParentRel(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

// ensureRemoteFolder 保证 relDir 在远端存在，返回其 folder_token。
// folderCache 既作为已有缓存（避免重复创建），也会被本函数填充新创建的 folder。
func ensureRemoteFolder(rootToken, relDir string, folderCache map[string]string, userToken string) (string, error) {
	if relDir == "" {
		return rootToken, nil
	}
	if tok, ok := folderCache[relDir]; ok {
		return tok, nil
	}
	parent := pushParentRel(relDir)
	parentToken, err := ensureRemoteFolder(rootToken, parent, folderCache, userToken)
	if err != nil {
		return "", err
	}
	name := relDir
	if i := strings.LastIndex(relDir, "/"); i >= 0 {
		name = relDir[i+1:]
	}
	tok, _, err := client.CreateFolder(name, parentToken, userToken)
	if err != nil {
		return "", err
	}
	folderCache[relDir] = tok
	return tok, nil
}

func init() {
	driveCmd.AddCommand(drivePushCmd)
	drivePushCmd.Flags().String("folder-token", "", "云盘根文件夹 token（必填）")
	drivePushCmd.Flags().String("local-dir", "", "本地根目录（必填）")
	drivePushCmd.Flags().String("if-exists", driveMirrorIfExistsSkip, "overwrite / skip（默认 skip 较安全）")
	drivePushCmd.Flags().Bool("delete-remote", false, "清理远端不存在于本地的文件（高危，需 --yes）")
	drivePushCmd.Flags().Bool("yes", false, "与 --delete-remote 配套确认删除")
	drivePushCmd.Flags().StringP("output", "o", "", "输出格式（json）")
	drivePushCmd.Flags().String("user-access-token", "", "User Access Token（覆盖登录态）")
	mustMarkFlagRequired(drivePushCmd, "folder-token")
	mustMarkFlagRequired(drivePushCmd, "local-dir")
}
