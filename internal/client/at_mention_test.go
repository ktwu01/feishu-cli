package client

import "testing"

func TestNormalizeAtMentions(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "id 属性无引号无斜杠",
			in:   `<at id=ou_alpha> hi`,
			want: `<at user_id="ou_alpha"> hi`,
		},
		{
			name: "open_id 属性带引号",
			in:   `<at open_id="ou_beta"> hello`,
			want: `<at user_id="ou_beta"> hello`,
		},
		{
			name: "user_id 自闭合无引号",
			in:   `<at user_id=ou_gamma /> bye`,
			want: `<at user_id="ou_gamma"> bye`,
		},
		{
			name: "email 形式保留不动（飞书原生支持）",
			in:   `<at email="alice@example.com"/> hi`,
			want: `<at email="alice@example.com"/> hi`,
		},
		{
			name: "@ all 透传",
			in:   `<at user_id="all"></at> attention`,
			want: `<at user_id="all"></at> attention`,
		},
		{
			name: "多种混合形式",
			in:   `<at id=ou_a/> 和 <at open_id="ou_b"> 还有 <at user_id=ou_c /> 以及 <at email="x@y.com"/>`,
			want: `<at user_id="ou_a"> 和 <at user_id="ou_b"> 还有 <at user_id="ou_c"> 以及 <at email="x@y.com"/>`,
		},
		{
			name: "无 @ 标签原样返回",
			in:   `普通文本 没有艾特`,
			want: `普通文本 没有艾特`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeAtMentions(c.in)
			if got != c.want {
				t.Errorf("NormalizeAtMentions() =\n  got:  %q\n  want: %q", got, c.want)
			}
		})
	}
}
