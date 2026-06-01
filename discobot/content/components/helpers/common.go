package helpers

func BoolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
