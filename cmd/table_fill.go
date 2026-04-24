package cmd

import (
	"fmt"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/converter"
)

// fillTableWithExtraRows 幂等地填充飞书表格：
//  1. 根据当前实际行数计算还缺多少行（重试场景不会重复追加，避免行数倍增）
//  2. 通过 insert_table_row API 追加缺失的扩展行
//  3. 填充所有单元格（FillTableCells 对同一 cell 写同值是幂等的）
//
// 被 doc import（processTableTask）和 doc add/content-update（fillTableWithRetry）共用。
// onAppendProgress 可为 nil；由调用方决定如何展示（syncPrintf / fmt.Printf）。
func fillTableWithExtraRows(
	documentID, tableBlockID string,
	td *converter.TableData,
	userAccessToken string,
	onAppendProgress client.InsertRowProgressFunc,
) error {
	if td.Cols <= 0 {
		return fmt.Errorf("表格列数 Cols=%d 不合法", td.Cols)
	}

	cellIDs, err := client.GetTableCellIDs(documentID, tableBlockID, userAccessToken)
	if err != nil {
		return fmt.Errorf("获取单元格失败: %w", err)
	}

	currentRows := len(cellIDs) / td.Cols
	targetRows := td.Rows + len(td.ExtraRowContents)

	// 幂等追加：仅追加"缺失"的行。若重试时发现已追加过，就不会再加。
	if currentRows < targetRows {
		missing := targetRows - currentRows
		if err := client.AppendTableRows(documentID, tableBlockID, missing, onAppendProgress, userAccessToken); err != nil {
			return err
		}
		cellIDs, err = client.GetTableCellIDs(documentID, tableBlockID, userAccessToken)
		if err != nil {
			return fmt.Errorf("获取追加后单元格失败: %w", err)
		}
	}

	// 填充初始单元格（幂等）
	initialCellCount := td.Rows * td.Cols
	if initialCellCount > len(cellIDs) {
		initialCellCount = len(cellIDs)
	}
	initialCellIDs := cellIDs[:initialCellCount]
	if len(td.CellElements) > 0 {
		if err := client.FillTableCellsRich(documentID, initialCellIDs, td.CellElements, td.CellContents, userAccessToken); err != nil {
			return fmt.Errorf("填充初始内容失败: %w", err)
		}
	} else if err := client.FillTableCells(documentID, initialCellIDs, td.CellContents, userAccessToken); err != nil {
		return fmt.Errorf("填充初始内容失败: %w", err)
	}

	if len(td.ExtraRowContents) == 0 {
		return nil
	}

	// 扁平化扩展行内容 + 填充
	extraContents, extraElements := flattenExtraRows(td)
	newCellIDs := cellIDs[initialCellCount:]
	if len(newCellIDs) < len(extraContents) {
		return fmt.Errorf("扩展行单元格不足: 实际 %d, 需要 %d", len(newCellIDs), len(extraContents))
	}
	newCellIDs = newCellIDs[:len(extraContents)]

	if len(extraElements) > 0 {
		if err := client.FillTableCellsRich(documentID, newCellIDs, extraElements, extraContents, userAccessToken); err != nil {
			return fmt.Errorf("填充扩展行失败: %w", err)
		}
		return nil
	}
	if err := client.FillTableCells(documentID, newCellIDs, extraContents, userAccessToken); err != nil {
		return fmt.Errorf("填充扩展行失败: %w", err)
	}
	return nil
}

// flattenExtraRows 将 TableData.ExtraRow{Contents,Elements} 从二维扁平化为 cell 数组。
// 返回的 elements 为 nil 时表示无富文本数据，调用方应使用纯文本填充路径。
func flattenExtraRows(td *converter.TableData) ([]string, [][]*larkdocx.TextElement) {
	n := len(td.ExtraRowContents)
	if n == 0 {
		return nil, nil
	}
	useRich := len(td.ExtraRowElements) > 0
	contents := make([]string, 0, n*td.Cols)
	var elements [][]*larkdocx.TextElement
	if useRich {
		elements = make([][]*larkdocx.TextElement, 0, n*td.Cols)
	}
	for i, row := range td.ExtraRowContents {
		contents = append(contents, row...)
		if useRich && i < len(td.ExtraRowElements) {
			elements = append(elements, td.ExtraRowElements[i]...)
		}
	}
	return contents, elements
}

// tableAppendProgress 返回用于 AppendTableRows 的进度回调；
// 当扩展行数 >= threshold 且 logger 非空时每 step 行打印一次，在最后一行也打印。
// logger("追加进度 %d/%d")。若条件不满足返回 nil（无回调）。
func tableAppendProgress(extraRowCount, threshold, step int, logger func(appended, total int)) client.InsertRowProgressFunc {
	if logger == nil || extraRowCount < threshold || step < 1 {
		return nil
	}
	return func(appended, total int) {
		if appended == total || appended%step == 0 {
			logger(appended, total)
		}
	}
}
