// Package theme defines the one semantic presentation token set for the terminal
// UI (ADD §12.3, contracts/theme.md) and the process-wide colour/TTY capability
// probe that selects the active variant. Tokens (Primary, Secondary, Success,
// Warning, Danger, Muted, Text) are lipgloss styles with dark and light
// variants; when colour is disabled every style degrades to plain while keeping
// bold. No renderer defines its own colour.
package theme
