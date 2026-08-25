package handlers

import "strings"

func pickDocPassword(header, form string) string {
	h := strings.TrimSpace(header)
	if h != "" {
		return h
	}
	return strings.TrimSpace(form)
}
