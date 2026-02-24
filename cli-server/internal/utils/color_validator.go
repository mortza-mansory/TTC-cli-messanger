package utils

import (
	"strings"
)

var validColors = map[string]bool{
	"[red]":     true,
	"[green]":   true,
	"[yellow]":  true,
	"[blue]":    true,
	"[magenta]": true,
	"[cyan]":    true,
	"[white]":   true,
	"[black]":   true,
	"":          true,
}

func IsValidColor(color string) bool {
	return validColors[color]
}

func NormalizeColor(color string) string {
	if color == "" {
		return "[white]"
	}

	if !strings.HasPrefix(color, "[") {
		color = "[" + color
	}
	if !strings.HasSuffix(color, "]") {
		color = color + "]"
	}

	if validColors[color] {
		return color
	}

	return "[white]"
}
