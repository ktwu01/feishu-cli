package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// fileExtToIMType maps common file extensions to Feishu IM file_type values.
// Falls back to "stream" for unknown extensions.
func fileExtToIMType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".opus":
		return "opus"
	case ".mp4":
		return "mp4"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "doc"
	case ".xls", ".xlsx":
		return "xls"
	case ".ppt", ".pptx":
		return "ppt"
	default:
		return "stream"
	}
}

// UploadIMFile uploads a local file via the IM API (/open-apis/im/v1/files)
// and returns the file_key that can be used directly in msg send --msg-type file.
func UploadIMFile(filePath string, fileName string) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	if fileName == "" {
		fileName = filepath.Base(filePath)
	}

	fileType := fileExtToIMType(fileName)

	req := larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType(fileType).
			FileName(fileName).
			File(f).
			Build()).
		Build()

	resp, err := client.Im.File.Create(Context(), req)
	if err != nil {
		return "", fmt.Errorf("上传文件失败: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("上传文件失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.FileKey == nil {
		return "", fmt.Errorf("上传文件成功但未返回 file_key")
	}

	return *resp.Data.FileKey, nil
}

// UploadIMImage uploads a local image via the IM API (/open-apis/im/v1/images)
// and returns the image_key that can be used directly in msg send --msg-type image.
func UploadIMImage(filePath string) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(f).
			Build()).
		Build()

	resp, err := client.Im.Image.Create(Context(), req)
	if err != nil {
		return "", fmt.Errorf("上传图片失败: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("上传图片失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.ImageKey == nil {
		return "", fmt.Errorf("上传图片成功但未返回 image_key")
	}

	return *resp.Data.ImageKey, nil
}
