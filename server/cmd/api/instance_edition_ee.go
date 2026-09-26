//go:build ee

package main

// serverAdminEnabled is false in Enterprise: signup, operators and
// application email are managed by the platform, not by a server admin.
const serverAdminEnabled = false
