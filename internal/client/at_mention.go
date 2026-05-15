package client

import "regexp"

// atMentionFixRe 匹配 AI 常见的 @ 标签变体：
//   <at id=ou_xxx>  /  <at open_id="ou_xxx">  /  <at user_id=ou_xxx/>
// 统一规范化为 <at user_id="ou_xxx"> 形式。
// <at email="..."/> 不在匹配范围内，会原样保留 —— 飞书 API 原生支持邮箱艾特。
var atMentionFixRe = regexp.MustCompile(`<at\s+(id|open_id|user_id)=("?)([^"\s/>]+)"?\s*/?>`)

// NormalizeAtMentions 修复 text / post 消息内容中常见的 @ 标签格式错误，
// 使其符合飞书 API 接受的 <at user_id="..."> 标准形式。
// 仅文本类内容应调用本函数；interactive 卡片 JSON 不应在此处理。
func NormalizeAtMentions(content string) string {
	return atMentionFixRe.ReplaceAllString(content, `<at user_id="$3">`)
}
