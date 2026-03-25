package client

import (
	"net/http"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

func TestBuildBitablePagePath(t *testing.T) {
	path := buildBitablePagePath("/open-apis/bitable/v1/apps/app_token/tables", 50, "next-token")
	expected := "/open-apis/bitable/v1/apps/app_token/tables?page_size=50&page_token=next-token"
	if path != expected {
		t.Errorf("buildBitablePagePath() = %q, 期望 %q", path, expected)
	}
}

func TestParseBitablePagedListResponse(t *testing.T) {
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusOK,
		RawBody:    []byte(`{"code":0,"msg":"ok","data":{"items":[{"table_id":"tbl_1","name":"主表"}],"page_token":"next_page","has_more":true}}`),
	}

	items, nextPageToken, err := parseBitablePagedListResponse[BitableTable](resp, "列出数据表")
	if err != nil {
		t.Fatalf("parseBitablePagedListResponse() 返回错误: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items 长度 = %d, 期望 1", len(items))
	}
	if items[0].TableID != "tbl_1" {
		t.Errorf("TableID = %q, 期望 %q", items[0].TableID, "tbl_1")
	}
	if nextPageToken != "next_page" {
		t.Errorf("nextPageToken = %q, 期望 %q", nextPageToken, "next_page")
	}
}

func TestParseBitablePagedListResponse_NoMore(t *testing.T) {
	resp := &larkcore.ApiResp{
		StatusCode: http.StatusOK,
		RawBody:    []byte(`{"code":0,"msg":"ok","data":{"items":[{"field_id":"fld_1","field_name":"状态","type":3}],"page_token":"ignored","has_more":false}}`),
	}

	items, nextPageToken, err := parseBitablePagedListResponse[BitableField](resp, "列出字段")
	if err != nil {
		t.Fatalf("parseBitablePagedListResponse() 返回错误: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items 长度 = %d, 期望 1", len(items))
	}
	if nextPageToken != "" {
		t.Errorf("nextPageToken = %q, 期望空字符串", nextPageToken)
	}
}
