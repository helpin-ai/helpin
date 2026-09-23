package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func envValues(data string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func readEnv(dir string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	return envValues(string(data)), err
}

func host(value string) string {
	u, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func validDomain(value string) bool {
	if len(value) > 253 || !strings.Contains(value, ".") || net.ParseIP(value) != nil {
		return false
	}
	label := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	for _, part := range strings.Split(value, ".") {
		if !label.MatchString(part) {
			return false
		}
	}
	return true
}

func (a *app) configuration(o *options, old map[string]string) error {
	fmt.Fprintln(a.out, "\nHelpin Community setup")
	defaultMode := "local"
	if strings.HasPrefix(old["APP_BASE_URL"], "https://") {
		defaultMode = "server"
	}
	if o.mode == "" {
		if o.yes {
			o.mode = defaultMode
		} else {
			var err error
			o.mode, err = a.ask("Where will Helpin run? local / server", defaultMode)
			if err != nil {
				return err
			}
		}
	}
	if o.mode != "local" && o.mode != "server" {
		return errors.New("--mode must be local or server")
	}
	for _, field := range []struct {
		value    *int
		key      string
		fallback int
	}{{&o.port, "DASHBOARD_PORT", 8085}, {&o.storagePort, "STORAGE_PORT", 9005}, {&o.helpPort, "HELPCENTER_PORT", 8086}} {
		if *field.value == 0 {
			*field.value = field.fallback
			if previous, err := strconv.Atoi(old[field.key]); err == nil {
				*field.value = previous
			}
		}
		if *field.value < 1024 || *field.value > 65535 {
			return errors.New("local service ports must be between 1024 and 65535")
		}
	}
	if o.port == o.storagePort || o.port == o.helpPort || o.storagePort == o.helpPort {
		return errors.New("dashboard, storage and help-center ports must be different")
	}
	if o.mode == "local" {
		return nil
	}
	if o.proxyMode == "" && o.proxy != "" {
		// An explicit trusted proxy address means the operator runs their own.
		o.proxyMode = "external"
	}
	if o.proxyMode == "" {
		fallback := old["HELPIN_PROXY"]
		if fallback == "" {
			// Existing server installations already run their own proxy.
			fallback = "builtin"
			if strings.HasPrefix(old["APP_BASE_URL"], "https://") {
				fallback = "external"
			}
		}
		if o.yes {
			o.proxyMode = fallback
		} else {
			value, err := a.ask("HTTPS: builtin (Helpin runs Caddy and gets certificates) / external (your own proxy)", fallback)
			if err != nil {
				return err
			}
			o.proxyMode = value
		}
	}
	if o.proxyMode != "builtin" && o.proxyMode != "external" {
		return errors.New("--proxy must be builtin or external")
	}
	fields := []struct {
		value           *string
		label, fallback string
	}{
		{&o.domain, "Dashboard hostname", host(old["APP_BASE_URL"])},
		{&o.storage, "Attachment hostname", host(old["PUBLIC_STORAGE_URL"])},
		{&o.help, "Help-center hostname", old["HELPIN_HELP_DOMAIN"]},
	}
	if o.proxyMode == "builtin" {
		// Caddy has a fixed address on its own network; nothing to guess.
		o.proxy = edgeProxyIP(old) + "/32"
		if o.acmeEmail == "" {
			o.acmeEmail = old["ACME_EMAIL"]
		}
	} else {
		trusted := old["COMMUNITY_TRUSTED_PROXY_CIDR"]
		if old["HELPIN_PROXY"] == "builtin" {
			trusted = ""
		}
		fields = append(fields, struct {
			value           *string
			label, fallback string
		}{&o.proxy, "Trusted proxy IP/CIDR (as seen by ingress)", trusted})
	}
	for _, field := range fields {
		if field.fallback == "localhost" {
			field.fallback = ""
		}
		if *field.value == "" {
			if o.yes {
				*field.value = field.fallback
			} else {
				value, err := a.ask(field.label, field.fallback)
				if err != nil {
					return err
				}
				*field.value = value
			}
		}
		if *field.value == "" {
			return fmt.Errorf("%s is required for server mode", field.label)
		}
	}
	for _, domain := range []string{o.domain, o.storage, o.help} {
		if !validDomain(domain) {
			return fmt.Errorf("invalid hostname %q; enter a DNS hostname without a scheme, port or path", domain)
		}
	}
	if strings.EqualFold(o.domain, o.storage) || strings.EqualFold(o.domain, o.help) || strings.EqualFold(o.storage, o.help) {
		return errors.New("dashboard, storage and help center need distinct hostnames")
	}
	if ip := net.ParseIP(o.proxy); ip != nil {
		if ip.To4() != nil {
			o.proxy += "/32"
		} else {
			o.proxy += "/128"
		}
	}
	_, network, err := net.ParseCIDR(o.proxy)
	if err != nil {
		return errors.New("--proxy-cidr must be the proxy source IP or CIDR as seen by ingress")
	}
	ones, _ := network.Mask.Size()
	if ones == 0 {
		return errors.New("trusting every proxy address is not allowed; use the actual proxy source IP/CIDR")
	}
	if o.acmeEmail != "" && !strings.Contains(o.acmeEmail, "@") {
		return errors.New("--acme-email must be an email address")
	}
	return nil
}

const defaultEdgeProxyIP = "172.30.255.2"

func edgeProxyIP(values map[string]string) string {
	if ip := net.ParseIP(values["EDGE_PROXY_IP"]); ip != nil && ip.To4() != nil {
		return ip.String()
	}
	return defaultEdgeProxyIP
}

// builtinCaddyfile serves the three public hostnames from the bundled Caddy,
// which reaches services by name on the edge network.
func builtinCaddyfile(o options) string {
	global := ""
	if o.acmeEmail != "" {
		global = fmt.Sprintf("{\n    email %s\n}\n\n", o.acmeEmail)
	}
	return global + fmt.Sprintf(`# Generated by helpin configure for the bundled Caddy proxy.
# Point these DNS names at this server and allow ports 80 and 443.
%s {
    encode zstd gzip
    reverse_proxy helpin-frontend:80
}
%s {
    encode zstd gzip
    reverse_proxy helpin-helpcenter:3000
}
%s {
    # Keep the Host header: signed file URLs depend on it.
    reverse_proxy garage:3900
}
`, o.domain, o.help, o.storage)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".helpin-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func writeConfiguration(dir string, o options) error {
	path := filepath.Join(dir, ".env")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	values := map[string]string{
		"DASHBOARD_PORT": strconv.Itoa(o.port), "STORAGE_PORT": strconv.Itoa(o.storagePort), "HELPCENTER_PORT": strconv.Itoa(o.helpPort),
		"BIND_ADDRESS": "127.0.0.1", "COMMUNITY_TRUSTED_PROXY_CIDR": "127.0.0.1/32",
		"APP_BASE_URL": fmt.Sprintf("http://localhost:%d", o.port), "PUBLIC_STORAGE_URL": fmt.Sprintf("http://localhost:%d", o.storagePort),
		"HELPIN_HELP_DOMAIN": "", "HELPIN_PROXY": "", "ACME_EMAIL": "",
	}
	if o.project != "" {
		values["COMPOSE_PROJECT_NAME"] = o.project
	}
	var caddy string
	if o.mode == "server" {
		values["APP_BASE_URL"] = "https://" + o.domain
		values["PUBLIC_STORAGE_URL"] = "https://" + o.storage
		values["COMMUNITY_TRUSTED_PROXY_CIDR"] = o.proxy
		values["HELPIN_HELP_DOMAIN"] = o.help
		values["HELPIN_PROXY"] = o.proxyMode
		if o.proxyMode == "builtin" {
			values["ACME_EMAIL"] = o.acmeEmail
			caddy = builtinCaddyfile(o)
		} else {
			caddy = fmt.Sprintf("# Generated by helpin configure. Run Caddy on this host.\n# Point DNS here, allow 80/443 and persist Caddy certificate storage.\n%s {\n    encode zstd gzip\n    reverse_proxy 127.0.0.1:%d\n}\n%s {\n    reverse_proxy 127.0.0.1:%d\n}\n%s {\n    reverse_proxy 127.0.0.1:%d\n}\n", o.domain, o.port, o.storage, o.storagePort, o.help, o.helpPort)
		}
	}
	values["PUBLIC_WIDGET_URL"] = values["APP_BASE_URL"]
	values["PUBLIC_SDK_URL"] = values["APP_BASE_URL"] + "/sdk/lib.js"
	lines := strings.Split(string(data), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		key, _, ok := strings.Cut(line, "=")
		if value, found := values[key]; ok && found {
			lines[i] = key + "=" + value
			seen[key] = true
		}
	}
	// Stable order keeps configuration reviewable.
	for _, key := range []string{"HELPIN_HELP_DOMAIN", "HELPIN_PROXY", "ACME_EMAIL"} {
		if value, ok := values[key]; ok && !seen[key] {
			lines = append(lines, key+"="+value)
			seen[key] = true
		}
	}
	for key := range values {
		if !seen[key] {
			return errors.New("bundle is missing required configuration keys; refusing a partial configuration update")
		}
	}
	if caddy != "" {
		if err = atomicWrite(filepath.Join(dir, "Caddyfile"), []byte(caddy), 0600); err != nil {
			return err
		}
	}
	return atomicWrite(path, []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n")+"\n"), 0600)
}
