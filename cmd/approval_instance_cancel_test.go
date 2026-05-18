package cmd

import "testing"

// TestApprovalInstanceCancelCmdRegistered 验证 cancel 子命令注册
func TestApprovalInstanceCancelCmdRegistered(t *testing.T) {
	if approvalInstanceCancelCmd.Use != "cancel" {
		t.Fatalf("Use = %q, want cancel", approvalInstanceCancelCmd.Use)
	}
	if approvalInstanceCancelCmd.Short == "" {
		t.Fatal("Short should not be empty")
	}
	found := false
	for _, sub := range approvalInstanceCmd.Commands() {
		if sub == approvalInstanceCancelCmd {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("approvalInstanceCancelCmd should be child of approvalInstanceCmd")
	}
}

// TestApprovalInstanceCancelFlagsDefaults 验证 flag 注册 + user-id-type 默认 open_id
func TestApprovalInstanceCancelFlagsDefaults(t *testing.T) {
	want := []string{"approval-code", "instance-code", "user-id", "user-id-type", "user-access-token"}
	for _, n := range want {
		if approvalInstanceCancelCmd.Flags().Lookup(n) == nil {
			t.Errorf("--%s missing", n)
		}
	}
	if u := approvalInstanceCancelCmd.Flags().Lookup("user-id-type"); u != nil && u.DefValue != "open_id" {
		t.Errorf("--user-id-type default=%q, want open_id", u.DefValue)
	}
}

// TestApprovalInstanceCancelRequiredFlags 验证必填
func TestApprovalInstanceCancelRequiredFlags(t *testing.T) {
	for _, n := range []string{"approval-code", "instance-code", "user-id"} {
		f := approvalInstanceCancelCmd.Flags().Lookup(n)
		if f == nil {
			t.Fatalf("--%s missing", n)
		}
		ann := f.Annotations["cobra_annotation_bash_completion_one_required_flag"]
		if len(ann) == 0 || ann[0] != "true" {
			t.Errorf("--%s should be required, ann=%v", n, ann)
		}
	}
}
