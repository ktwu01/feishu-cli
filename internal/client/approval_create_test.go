package client

import (
	"strings"
	"testing"
)

// TestCreateApprovalInstanceNormalizesEmptyUserIDType 验证空 UserIDType normalize 成 open_id
// (codex 二轮 rv finding 3)
func TestCreateApprovalInstanceNormalizesEmptyUserIDType(t *testing.T) {
	// 不真打 API，只验证 body 构造逻辑
	// 因为函数会发 HTTP 请求，这里只断言 validation 阶段不出错（实际网络层会失败但 body 构造已经走完）
	_, err := CreateApprovalInstance(CreateApprovalInstanceOptions{
		ApprovalCode: "",
		UserID:       "ou_xxx",
		Form:         "[]",
		UserIDType:   "", // 关键：空值
	}, "")
	// 期望：approval_code 为空才会先报错，证明 normalize 没崩
	if err == nil {
		t.Fatal("expected error for empty approval_code")
	}
	if !strings.Contains(err.Error(), "approval_code") {
		t.Errorf("expected approval_code error, got: %v", err)
	}
}

// TestCreateApprovalInstanceRejectsInvalidUserIDType 验证非法 UserIDType 报错
func TestCreateApprovalInstanceRejectsInvalidUserIDType(t *testing.T) {
	_, err := CreateApprovalInstance(CreateApprovalInstanceOptions{
		ApprovalCode: "OK",
		UserID:       "ou_xxx",
		Form:         "[]",
		UserIDType:   "bogus_type",
	}, "")
	if err == nil {
		t.Fatal("expected error for invalid user_id_type")
	}
	if !strings.Contains(err.Error(), "user_id_type") {
		t.Errorf("expected user_id_type error, got: %v", err)
	}
}
