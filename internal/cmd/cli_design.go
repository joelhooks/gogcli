package cmd

import (
	"context"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/steipete/gogcli/internal/config"
	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/secrets"
)

const (
	maxNextActions = 8
	maxRootActions = 32
)

var aliasGroupPattern = regexp.MustCompile(`\s+\([^)]*\)`)

func defaultOutputMode(flags RootFlags) outfmt.Mode {
	// Agent-first default: JSON unless plain output is explicitly requested.
	if flags.Plain {
		return outfmt.Mode{Plain: true}
	}

	return outfmt.Mode{JSON: true}
}

func fallbackOutputMode(args []string) outfmt.Mode {
	if hasAnyFlag(args, "--plain", "--tsv", "-p") {
		return outfmt.Mode{Plain: true}
	}

	return outfmt.Mode{JSON: true}
}

func commandString(args []string) string {
	if len(args) == 0 {
		return "gog"
	}

	return "gog " + strings.Join(args, " ")
}

func wantsRootCommandTree(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if len(args) != 1 {
		return false
	}

	switch strings.TrimSpace(args[0]) {
	case "--help", "-h", "help":
		return true
	default:
		return false
	}
}

func hasAnyFlag(args []string, flags ...string) bool {
	want := make(map[string]struct{}, len(flags))
	for _, flag := range flags {
		want[flag] = struct{}{}
	}
	for _, arg := range args {
		if _, ok := want[arg]; ok {
			return true
		}
	}
	return false
}

func commandPathWithRoot(node *kong.Node) string {
	if node == nil {
		return "gog"
	}

	path := stripAliasGroups(strings.TrimSpace(node.FullPath()))
	if path == "" {
		return "gog"
	}
	return path
}

func commandTemplateWithRoot(node *kong.Node) string {
	if node == nil {
		return "gog"
	}

	summary := stripAliasGroups(strings.TrimSpace(node.Summary()))
	if summary == "" {
		return commandPathWithRoot(node)
	}
	if strings.HasPrefix(summary, "gog ") {
		return summary
	}

	return "gog " + summary
}

func stripAliasGroups(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return strings.TrimSpace(aliasGroupPattern.ReplaceAllString(s, ""))
}

func visibleCommandChildren(node *kong.Node) []*kong.Node {
	if node == nil {
		return nil
	}

	children := make([]*kong.Node, 0, len(node.Children))
	for _, child := range node.Children {
		if child == nil || child.Type != kong.CommandNode || child.Hidden {
			continue
		}
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
	})
	return children
}

func nextActionsForNode(node *kong.Node) []outfmt.NextAction {
	actions := make([]outfmt.NextAction, 0, maxNextActions)

	if node == nil {
		return []outfmt.NextAction{
			{
				Command:     "gog schema",
				Description: "Inspect the full machine-readable command schema",
			},
			{
				Command:     "gog version",
				Description: "Print CLI build information",
			},
		}
	}

	schemaCmd := "gog schema"
	if path := strings.TrimSpace(strings.TrimPrefix(commandPathWithRoot(node), "gog")); path != "" {
		schemaCmd = "gog schema " + strings.TrimSpace(path)
	}
	actions = append(actions, outfmt.NextAction{
		Command:     schemaCmd,
		Description: "Inspect machine-readable schema for this command",
	})

	for _, child := range visibleCommandChildren(node) {
		help := strings.TrimSpace(child.Help)
		if help == "" {
			help = "Run subcommand"
		}

		actions = append(actions, outfmt.NextAction{
			Command:     commandTemplateWithRoot(child),
			Description: help,
		})
		if len(actions) >= maxNextActions {
			return dedupeNextActions(actions)
		}
	}

	if node.Parent != nil && node.Parent.Type == kong.CommandNode {
		actions = append(actions, outfmt.NextAction{
			Command:     commandPathWithRoot(node.Parent) + " --help",
			Description: "Show parent command help",
		})
	}

	actions = append(actions, outfmt.NextAction{
		Command:     "gog version",
		Description: "Print CLI build information",
	})

	return dedupeNextActions(actions)
}

func dedupeNextActions(actions []outfmt.NextAction) []outfmt.NextAction {
	seen := make(map[string]struct{}, len(actions))
	out := make([]outfmt.NextAction, 0, len(actions))
	for _, action := range actions {
		key := strings.TrimSpace(action.Command)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, action)
	}
	return out
}

func rootCommandTree(root *kong.Node) map[string]any {
	configPath, cfgErr := config.ConfigPath()
	configFile := "unknown"
	if cfgErr == nil && strings.TrimSpace(configPath) != "" {
		configFile = configPath
	}

	keyringBackend := "unknown"
	keyringSource := "unknown"
	backendInfo, backendErr := secrets.ResolveKeyringBackendInfo()
	if backendErr == nil {
		keyringBackend = backendInfo.Value
		keyringSource = backendInfo.Source
	}

	commands := make([]map[string]any, 0, maxRootActions)
	for _, child := range visibleCommandChildren(root) {
		entry := map[string]any{
			"name":        child.Name,
			"description": strings.TrimSpace(child.Help),
			"usage":       commandTemplateWithRoot(child),
		}
		if len(child.Aliases) > 0 {
			aliases := make([]string, len(child.Aliases))
			copy(aliases, child.Aliases)
			sort.Strings(aliases)
			entry["aliases"] = aliases
		}
		commands = append(commands, entry)
		if len(commands) >= maxRootActions {
			break
		}
	}

	return map[string]any{
		"description": baseDescription(),
		"build":       VersionString(),
		"config": map[string]any{
			"file":            configFile,
			"keyring_backend": keyringBackend,
			"keyring_source":  keyringSource,
		},
		"commands": commands,
	}
}

func writeRootCommandTree(args []string, root *kong.Node) error {
	ctx := context.Background()
	mode := fallbackOutputMode(args)
	ctx = outfmt.WithMode(ctx, mode)
	ctx = outfmt.WithEnvelope(ctx, true)
	ctx = outfmt.WithCommand(ctx, commandString(args))
	ctx = outfmt.WithNextActions(ctx, nextActionsForNode(root))

	return outfmt.WriteJSON(ctx, os.Stdout, rootCommandTree(root))
}

func exitCodeString(code int) string {
	switch code {
	case 2:
		return "USAGE_ERROR"
	case emptyResultsExitCode:
		return "EMPTY_RESULTS"
	case exitCodeAuthRequired:
		return "AUTH_REQUIRED"
	case exitCodeNotFound:
		return "NOT_FOUND"
	case exitCodePermissionDenied:
		return "PERMISSION_DENIED"
	case exitCodeRateLimited:
		return "RATE_LIMITED"
	case exitCodeRetryable:
		return "RETRYABLE"
	case exitCodeConfig:
		return "CONFIG_ERROR"
	case exitCodeCancelled:
		return "CANCELLED"
	default:
		return "COMMAND_FAILED"
	}
}

func fixForExitCode(code int) string {
	switch code {
	case 2:
		return "Check required args/flags and inspect command structure with `gog schema`."
	case emptyResultsExitCode:
		return "Widen filters/date range or increase page size and retry."
	case exitCodeAuthRequired:
		return "Authenticate with `gog auth add <email>` and rerun with `--account <email>`."
	case exitCodeConfig:
		return "Set OAuth credentials using `gog auth credentials <path-to-credentials.json>`."
	case exitCodePermissionDenied:
		return "Verify account permissions and OAuth scopes for the requested API."
	case exitCodeRateLimited, exitCodeRetryable:
		return "Retry with backoff; if persistent, reduce request volume."
	case exitCodeNotFound:
		return "Verify IDs/names and confirm the resource exists for the selected account."
	case exitCodeCancelled:
		return "Rerun the command without interruption."
	default:
		return "Run `gog schema` or `gog --help` to inspect supported command syntax."
	}
}
