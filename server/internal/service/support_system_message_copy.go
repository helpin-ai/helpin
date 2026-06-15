package service

import "strings"

func supportSystemFirstName(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func supportSystemActorName(name, fallback string) string {
	if first := supportSystemFirstName(name); first != "" {
		return first
	}
	return fallback
}
