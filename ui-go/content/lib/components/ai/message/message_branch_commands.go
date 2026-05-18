package message

import "net/url"

func branchCommand(messageID string, direction string) string {
	values := url.Values{}
	values.Set("message_id", messageID)
	values.Set("direction", direction)
	return "@post('/ui/commands/message/branch?" + values.Encode() + "')"
}
