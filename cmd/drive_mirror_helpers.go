package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/riba2534/feishu-cli/internal/client"
)

// resolveSafeLocalDir 把用户传入的 --local-dir 解为「完全解析符号链接 + 限定在 cwd 子树」的绝对路径。
// 这是 drive pull/push/status 的安全前置：避免 `link/..` 这类路径在 walk 时被内核解析到 cwd 之外。
func resolveSafeLocalDir(localDir string) (safeAbs, cwdAbs string, err error) {
	if localDir == "" {
		return "", "", fmt.Errorf("--local-dir 不能为空")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("获取 cwd 失败: %w", err)
	}
	cwdAbs, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		// EvalSymlinks 失败时退化到 cwd 本身（没有 symlink 也是合理）
		cwdAbs = cwd
	}

	// 先确保目录存在
	info, statErr := os.Stat(localDir)
	if statErr != nil {
		return "", "", fmt.Errorf("--local-dir 不存在或无法访问: %w", statErr)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("--local-dir 不是目录: %s", localDir)
	}

	abs, err := filepath.Abs(localDir)
	if err != nil {
		return "", "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// 符号链接解析失败，退回 abs 但不允许越界
		resolved = abs
	}

	rel, err := filepath.Rel(cwdAbs, resolved)
	if err != nil || rel == ".." || (len(rel) >= 3 && rel[:3] == "../") {
		return "", "", fmt.Errorf("--local-dir 必须在当前工作目录子树内: %s", localDir)
	}
	return resolved, cwdAbs, nil
}

// walkLocalRegularFiles 走 root，返回 rel_path（用 / 分隔，相对 root）→ 绝对路径 的映射。
// 仅收 regular file，不跟随子级 symlink。
func walkLocalRegularFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(absPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, absPath)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = absPath
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历 %s 失败: %w", root, err)
	}
	return files, nil
}

// walkLocalDirs 返回 root 下的所有子目录（rel_path，不含 root 本身）。
// 用于 push 阶段镜像目录结构。
func walkLocalDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(absPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() || absPath == root {
			return nil
		}
		rel, err := filepath.Rel(root, absPath)
		if err != nil {
			return err
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历 %s 失败: %w", root, err)
	}
	return dirs, nil
}

// remoteFilesOnly 从全量 entry map 提取 type=file 的子集（ rel_path → file_token）。
func remoteFilesOnly(entries map[string]client.DriveRemoteEntry) map[string]string {
	out := make(map[string]string, len(entries))
	for rel, e := range entries {
		if e.Type == "file" {
			out[rel] = e.FileToken
		}
	}
	return out
}
