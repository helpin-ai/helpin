//go:build !ee

package main

// serverAdminEnabled turns on self-hosted server administration: the first
// account becomes the server admin, a signup policy, and application email
// settings saved in the app.
const serverAdminEnabled = true
