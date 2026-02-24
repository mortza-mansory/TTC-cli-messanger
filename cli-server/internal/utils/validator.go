package utils

import "strings"

func ValidateMessage(sender, content string) bool {
	if strings.TrimSpace(sender) == "" {
		return false
	}
	if strings.TrimSpace(content) == "" {
		return false
	}
	if len(content) > 10000 {
		return false
	}
	return true
}
