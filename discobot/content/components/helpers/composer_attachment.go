package helpers

import "net/url"

func ComposerAttachmentRemoveCommand(attachmentID string) string {
	return "@discobotCommand('/ui/commands/composer/attachments/" + url.PathEscape(attachmentID) + "/remove', {method: 'POST'})"
}
