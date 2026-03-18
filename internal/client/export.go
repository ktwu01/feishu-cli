package client

import (
	"fmt"
	"time"

	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

// CreateExportTask 创建导出任务，返回任务 ticket
func CreateExportTask(docToken, docType, fileExtension, userAccessToken string) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	exportTask := larkdrive.NewExportTaskBuilder().
		Token(docToken).
		Type(docType).
		FileExtension(fileExtension).
		Build()

	req := larkdrive.NewCreateExportTaskReqBuilder().
		ExportTask(exportTask).
		Build()

	resp, err := client.Drive.ExportTask.Create(Context(), req, UserTokenOption(userAccessToken)...)
	if err != nil {
		return "", fmt.Errorf("创建导出任务失败: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("创建导出任务失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Ticket == nil {
		return "", fmt.Errorf("创建导出任务成功但未返回 ticket")
	}

	return *resp.Data.Ticket, nil
}

// GetExportTask 查询导出任务状态，返回 jobStatus、fileToken、error
// jobStatus: 0=成功, 1=初始化, 2=处理中
func GetExportTask(ticket, docToken, userAccessToken string) (int, string, error) {
	client, err := GetClient()
	if err != nil {
		return -1, "", err
	}

	req := larkdrive.NewGetExportTaskReqBuilder().
		Ticket(ticket).
		Token(docToken).
		Build()

	resp, err := client.Drive.ExportTask.Get(Context(), req, UserTokenOption(userAccessToken)...)
	if err != nil {
		return -1, "", fmt.Errorf("查询导出任务失败: %w", err)
	}

	if !resp.Success() {
		return -1, "", fmt.Errorf("查询导出任务失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Result == nil {
		return -1, "", fmt.Errorf("查询导出任务未返回结果")
	}

	result := resp.Data.Result
	jobStatus := IntVal(result.JobStatus)
	fileToken := StringVal(result.FileToken)

	// jobStatus 0=成功, 1=初始化, 2=处理中, 其他=失败
	if jobStatus != 0 && jobStatus != 1 && jobStatus != 2 {
		errMsg := "未知错误"
		if result.JobErrorMsg != nil && *result.JobErrorMsg != "" {
			errMsg = *result.JobErrorMsg
		}
		return jobStatus, "", fmt.Errorf("导出任务失败: %s", errMsg)
	}

	return jobStatus, fileToken, nil
}

// DownloadExportFile 下载导出任务生成的文件
func DownloadExportFile(fileToken, outputPath, userAccessToken string) error {
	if err := validatePath(outputPath); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	req := larkdrive.NewDownloadExportTaskReqBuilder().
		FileToken(fileToken).
		Build()

	resp, err := client.Drive.ExportTask.Download(ContextWithTimeout(downloadTimeout), req, UserTokenOption(userAccessToken)...)
	if err != nil {
		return fmt.Errorf("下载导出文件失败: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("下载导出文件失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return saveToFile(resp.File, outputPath)
}

// WaitExportTask 轮询等待导出任务完成，返回导出文件的 fileToken
func WaitExportTask(ticket, docToken, userAccessToken string, maxRetries int) (string, error) {
	for i := 0; i < maxRetries; i++ {
		jobStatus, fileToken, err := GetExportTask(ticket, docToken, userAccessToken)
		if err != nil {
			return "", err
		}

		if jobStatus == 0 {
			return fileToken, nil
		}

		// jobStatus 1=初始化, 2=处理中
		time.Sleep(1 * time.Second)
	}

	return "", fmt.Errorf("导出任务超时，已等待 %d 秒", maxRetries)
}

// CreateImportTask 创建导入任务，返回任务 ticket
func CreateImportTask(fileToken, fileType, fileName, targetType, folderToken string) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	taskBuilder := larkdrive.NewImportTaskBuilder().
		FileExtension(fileType).
		FileToken(fileToken).
		Type(targetType)

	if fileName != "" {
		taskBuilder.FileName(fileName)
	}

	if folderToken != "" {
		taskBuilder.Point(larkdrive.NewImportTaskMountPointBuilder().
			MountType(1).
			MountKey(folderToken).
			Build())
	}

	req := larkdrive.NewCreateImportTaskReqBuilder().
		ImportTask(taskBuilder.Build()).
		Build()

	resp, err := client.Drive.ImportTask.Create(Context(), req)
	if err != nil {
		return "", fmt.Errorf("创建导入任务失败: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("创建导入任务失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Ticket == nil {
		return "", fmt.Errorf("创建导入任务成功但未返回 ticket")
	}

	return *resp.Data.Ticket, nil
}

// GetImportTask 查询导入任务状态，返回 jobStatus、docToken、url、error
// jobStatus: 0=成功, 1=初始化, 2=处理中
func GetImportTask(ticket string) (int, string, string, error) {
	client, err := GetClient()
	if err != nil {
		return -1, "", "", err
	}

	req := larkdrive.NewGetImportTaskReqBuilder().
		Ticket(ticket).
		Build()

	resp, err := client.Drive.ImportTask.Get(Context(), req)
	if err != nil {
		return -1, "", "", fmt.Errorf("查询导入任务失败: %w", err)
	}

	if !resp.Success() {
		return -1, "", "", fmt.Errorf("查询导入任务失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Result == nil {
		return -1, "", "", fmt.Errorf("查询导入任务未返回结果")
	}

	result := resp.Data.Result
	jobStatus := IntVal(result.JobStatus)
	docToken := StringVal(result.Token)
	url := StringVal(result.Url)

	// jobStatus 0=成功, 1=初始化, 2=处理中, 其他=失败
	if jobStatus != 0 && jobStatus != 1 && jobStatus != 2 {
		errMsg := "未知错误"
		if result.JobErrorMsg != nil && *result.JobErrorMsg != "" {
			errMsg = *result.JobErrorMsg
		}
		return jobStatus, "", "", fmt.Errorf("导入任务失败: %s", errMsg)
	}

	return jobStatus, docToken, url, nil
}

// WaitImportTask 轮询等待导入任务完成，返回文档 token 和 url
func WaitImportTask(ticket string, maxRetries int) (string, string, error) {
	for i := 0; i < maxRetries; i++ {
		jobStatus, docToken, url, err := GetImportTask(ticket)
		if err != nil {
			return "", "", err
		}

		if jobStatus == 0 {
			return docToken, url, nil
		}

		time.Sleep(1 * time.Second)
	}

	return "", "", fmt.Errorf("导入任务超时，已等待 %d 秒", maxRetries)
}
