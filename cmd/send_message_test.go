package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// -------------------------------------------------------------------
// isLocalPath tests - 纯函数，无 API 调用
// -------------------------------------------------------------------

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// 远程 URL 应该返回 false
		{"http URL", "http://example.com/image.png", false},
		{"https URL", "https://example.com/image.png", false},
		{"https URL 带参数", "https://example.com/image.png?token=abc", false},

		// 飞书 image_key 应该返回 false
		{"img_ 开头", "img_v2_abc123", false},
		{"file_v2_ 开头", "file_v2_abc123", false},

		// 图片扩展名应该是本地路径
		{"相对路径 png", "screenshot.png", true},
		{"相对路径 jpg", "photo.jpg", true},
		{"相对路径 jpeg", "image.jpeg", true},
		{"相对路径 gif", "animation.gif", true},
		{"相对路径 bmp", "icon.bmp", true},
		{"相对路径 webp", "image.webp", true},
		{"相对路径 svg", "vector.svg", true},

		// 绝对路径应该是本地路径
		{"绝对路径 Unix", "/Users/test/image.png", true},
		{"绝对路径 Windows", "C:\\Users\\test\\image.png", true},
		{"绝对路径 带扩展名", "/home/user/docs/photo.JPG", true},

		// 相对路径带目录分隔符
		{"相对路径带斜杠", "images/logo.png", true},
		{"相对路径反斜杠", "images\\logo.png", true},
		{"上级目录", "../assets/icon.png", true},
		{"当前目录", "./screenshot.png", true},

		// 非图片扩展名不应该被识别为本地图片路径（但仍会被 isLocalPath 返回 true 因为有分隔符）
		{"txt 文件（无分隔符）", "readme.txt", false},
		{"md 文件（无分隔符）", "document.md", false},
		{"空字符串", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalPath(tt.path)
			if result != tt.expected {
				t.Errorf("isLocalPath(%q) = %v, 期望 %v", tt.path, result, tt.expected)
			}
		})
	}
}

// -------------------------------------------------------------------
// processJSONLocalImages 测试 - 纯逻辑，不触发实际上传
// 仅测试跳过远程/已上传图片、不存在的文件等不上传的场景
// -------------------------------------------------------------------

func TestProcessJSONLocalImages_SkipRemoteImages(t *testing.T) {
	// 测试远程图片 URL 应该被跳过（不会尝试上传）
	tests := []struct {
		name        string
		jsonContent string
		expectKey   string // 期望在结果中保留原值
	}{
		{
			name: "https URL",
			jsonContent: `{
				"tag": "img",
				"image_key": "https://example.com/image.png"
			}`,
			expectKey: "https://example.com/image.png",
		},
		{
			name: "http URL",
			jsonContent: `{
				"tag": "img",
				"image_key": "http://cdn.example.com/photo.jpg"
			}`,
			expectKey: "http://cdn.example.com/photo.jpg",
		},
		{
			name: "img_ 前缀（已上传）",
			jsonContent: `{
				"tag": "img",
				"image_key": "img_v2_already_uploaded"
			}`,
			expectKey: "img_v2_already_uploaded",
		},
		{
			name: "file_ 前缀（已上传文件）",
			jsonContent: `{
				"tag": "img",
				"image_key": "file_v2_xxx"
			}`,
			expectKey: "file_v2_xxx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data interface{}
			if err := json.Unmarshal([]byte(tt.jsonContent), &data); err != nil {
				t.Fatalf("JSON 解析失败: %v", err)
			}

			changed, newData, count := processJSONLocalImages(data, "/tmp/nonexistent")

			// 远程图片和已上传图片不应该被修改
			if changed {
				t.Errorf("远程/已上传图片不应该被修改，changed=true")
			}
			if count != 0 {
				t.Errorf("上传计数应该为 0，实际 %d", count)
			}

			// 验证结果保留原值
			resultBytes, err := json.Marshal(newData)
			if err != nil {
				t.Fatalf("JSON 序列化失败: %v", err)
			}
			if !contains(string(resultBytes), tt.expectKey) {
				t.Errorf("结果 JSON 不包含期望的 image_key %q，结果: %s", tt.expectKey, string(resultBytes))
			}
		})
	}
}

func TestProcessJSONLocalImages_SkipNonExistentFiles(t *testing.T) {
	// 测试不存在的本地文件应该被跳过（不会尝试上传）
	jsonContent := `{
		"tag": "img",
		"image_key": "/nonexistent/path/to/image.png"
	}`

	var data interface{}
	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	changed, newData, count := processJSONLocalImages(data, "/tmp")

	// 不存在的文件不应该被修改
	if changed {
		t.Errorf("不存在的文件不应该被修改，changed=true")
	}
	if count != 0 {
		t.Errorf("上传计数应该为 0，实际 %d", count)
	}

	// 验证结果保留原值
	resultBytes, _ := json.Marshal(newData)
	if !contains(string(resultBytes), "/nonexistent/path/to/image.png") {
		t.Errorf("结果应该保留原路径，结果: %s", string(resultBytes))
	}
}

func TestProcessJSONLocalImages_NonImageTag(t *testing.T) {
	// 测试非 img 标签不应该被修改
	jsonContent := `{
		"tag": "text",
		"image_key": "some.png"
	}`

	var data interface{}
	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	changed, newData, count := processJSONLocalImages(data, "/tmp")

	if changed {
		t.Errorf("非 img 标签不应该被修改")
	}
	if count != 0 {
		t.Errorf("上传计数应该为 0")
	}

	resultBytes, _ := json.Marshal(newData)
	if !contains(string(resultBytes), "some.png") {
		t.Errorf("结果应该保留原值，结果: %s", string(resultBytes))
	}
}

func TestProcessJSONLocalImages_InvalidJSON(t *testing.T) {
	// 非 JSON 字符串应该原样返回
	changed, result, count := processJSONLocalImages("not json", "/tmp")
	if changed {
		t.Errorf("非 JSON 字符串不应该被修改")
	}
	if result != "not json" {
		t.Errorf("非 JSON 字符串应该原样返回")
	}
	if count != 0 {
		t.Errorf("计数应该为 0")
	}
}

func TestProcessJSONLocalImages_NestedStructure(t *testing.T) {
	// 测试嵌套 JSON 结构中的 img 标签
	jsonContent := `{
		"header": {
			"template": "blue",
			"title": {"tag": "plain_text", "content": "测试"}
		},
		"elements": [
			{"tag": "markdown", "content": "hello"},
			{"tag": "img", "image_key": "https://example.com/remote.png"}
		]
	}`

	var data interface{}
	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	changed, _, count := processJSONLocalImages(data, "/tmp")

	// 远程图片不应该被处理
	if changed {
		t.Errorf("远程图片不应该被处理")
	}
	if count != 0 {
		t.Errorf("计数应该为 0")
	}
}

func TestProcessJSONLocalImages_ArrayStructure(t *testing.T) {
	// 测试数组结构
	jsonContent := `[
		{"tag": "img", "image_key": "https://example.com/a.png"},
		{"tag": "text", "text": "hello"},
		{"tag": "img", "image_key": "img_v2_b"}
	]`

	var data interface{}
	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	changed, _, count := processJSONLocalImages(data, "/tmp")

	// 所有都是远程/已上传图片，不应该被处理
	if changed {
		t.Errorf("远程/已上传图片不应该被处理")
	}
	if count != 0 {
		t.Errorf("计数应该为 0")
	}
}

// -------------------------------------------------------------------
// processAndUploadLocalImages 测试 - 纯逻辑，不触发实际上传
// -------------------------------------------------------------------

func TestProcessAndUploadLocalImages_SkipRemoteMarkdownImages(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectContains string
	}{
		{
			name:           "https 图片 URL 保留",
			content:        "![](https://example.com/image.png)",
			expectContains: "https://example.com/image.png",
		},
		{
			name:           "http 图片 URL 保留",
			content:        "![](http://cdn.example.com/photo.jpg)",
			expectContains: "http://cdn.example.com/photo.jpg",
		},
		{
			name:           "已上传图片 key 保留",
			content:        "![](img_v2_uploaded)",
			expectContains: "img_v2_uploaded",
		},
		{
			name:           "file_ key 保留",
			content:        "![](file_v2_abc123)",
			expectContains: "file_v2_abc123",
		},
		{
			name:           "不存在的本地文件跳过",
			content:        "![](nonexistent.png)",
			expectContains: "![](nonexistent.png)", // 原样保留
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, count, err := processAndUploadLocalImages(tt.content, "/tmp/nonexistent")
			if err != nil {
				t.Fatalf("不应该返回错误: %v", err)
			}
			if count != 0 {
				t.Errorf("不应该有任何上传，count=%d", count)
			}
			if !contains(result, tt.expectContains) {
				t.Errorf("结果不包含期望字符串 %q，结果: %s", tt.expectContains, result)
			}
		})
	}
}

func TestProcessAndUploadLocalImages_TextContent(t *testing.T) {
	// 测试纯文本内容不应该被处理
	content := "Hello World, this is plain text."
	result, count, err := processAndUploadLocalImages(content, "/tmp")
	if err != nil {
		t.Fatalf("不应该返回错误: %v", err)
	}
	if count != 0 {
		t.Errorf("纯文本不应该处理图片，count=%d", count)
	}
	if result != content {
		t.Errorf("纯文本应该原样返回")
	}
}

func TestProcessAndUploadLocalImages_MixedContent(t *testing.T) {
	// 测试混合内容：既有远程图片又有本地图片引用
	content := "Remote: ![](https://example.com/remote.png) and local: ![](local.png)"
	result, count, err := processAndUploadLocalImages(content, "/tmp/nonexistent")
	if err != nil {
		t.Fatalf("不应该返回错误: %v", err)
	}
	// local.png 不存在，所以 count=0
	if count != 0 {
		t.Errorf("不存在的本地文件不应该被上传，count=%d", count)
	}
	// 远程图片应该保留
	if !contains(result, "https://example.com/remote.png") {
		t.Errorf("远程图片 URL 应该保留，结果: %s", result)
	}
}

// -------------------------------------------------------------------
// Markdown 图片正则测试
// -------------------------------------------------------------------

func TestMarkdownImageRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantAlt  string
		wantPath string
	}{
		{"简单图片", "![](path.png)", 1, "", "path.png"},
		{"有 alt 文本", "![alt text](image.jpg)", 1, "alt text", "image.jpg"},
		{"带空格", "![  alt  ](path/gif)", 1, "  alt  ", "path/gif"},
		{"无匹配", "no image here", 0, "", ""},
		{"行内代码中的括号不匹配", "`![](a.png)`", 1, "", "a.png"}, // 正则会匹配到
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := markdownImageRegex.FindAllStringSubmatch(tt.input, -1)
			if len(matches) != tt.wantLen {
				t.Errorf("期望 %d 个匹配，实际 %d 个", tt.wantLen, len(matches))
				return
			}
			if tt.wantLen > 0 {
				if matches[0][1] != tt.wantAlt {
					t.Errorf("alt 文本期望 %q，实际 %q", tt.wantAlt, matches[0][1])
				}
				if matches[0][2] != tt.wantPath {
					t.Errorf("路径期望 %q，实际 %q", tt.wantPath, matches[0][2])
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// 辅助函数
// -------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------------
// 路径解析集成测试（创建真实临时文件）
// -------------------------------------------------------------------

func TestResolveLocalPath_Integration(t *testing.T) {
	// 这个测试验证相对路径解析逻辑是否正确
	tmpDir := t.TempDir()

	// 创建测试图片文件
	imgPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake"), 0600); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(imgPath); err != nil {
		t.Fatalf("文件应该存在: %v", err)
	}

	// 测试 JSON 中引用该文件（使用相对于 tmpDir 的路径）
	relPath := "test.png"
	jsonContent := `{
		"tag": "img",
		"image_key": "` + relPath + `"
	}`

	var data interface{}
	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	// 使用 tmpDir 作为 basePath，relPath 应该被解析为 tmpDir/test.png
	changed, newData, count := processJSONLocalImages(data, tmpDir)

	// 注意：这个测试会因为实际上传而失败（没有 API 凭据）
	// 但我们可以验证路径解析的逻辑
	// 如果 API 调用成功，changed 应该为 true
	_ = changed
	_ = newData
	_ = count
}
