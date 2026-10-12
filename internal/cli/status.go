package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"unicode"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workcontext"
	"github.com/spf13/cobra"
)

// statusView is a transient presentation of saved canonical context.
type statusView struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Branch   string            `json:"branch"`
	Location string            `json:"location"`
	Links    map[string]string `json:"links"`
}

func newStatusCmd() *cobra.Command {
	var machine bool
	cmd := &cobra.Command{
		Use:   "status [WORK]",
		Short: "Inspect saved context for the current or named Work",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return diag.New(diag.Usage, "use `work status [WORK] [--json]` with at most one complete Work id")
			}
			return nil
		},
		SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "" {
				return diag.New(diag.TargetNotFound, "no Work has an empty id; enter a Work directory or supply its complete id")
			}
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			c, err := resolveContext(cmd.Context(), target)
			if err != nil {
				return err
			}
			jsonOutput := machine
			if !cmd.Flags().Changed("json") {
				jsonOutput, _ = cmd.Root().PersistentFlags().GetBool("json")
			}
			data, err := serializeStatus(c, jsonOutput)
			if err != nil {
				return err
			}
			if err := cmd.Context().Err(); err != nil {
				return diag.Wrap(diag.Cancelled, err, "status cancelled")
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	// This local visible flag shadows the hidden inherited read flag.
	cmd.Flags().BoolVar(&machine, "json", false, "emit saved context as JSON")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return diag.Wrap(diag.Usage, err, "invalid status option; use `work status [WORK] [--json]`")
	})
	return cmd
}

func serializeStatus(c workcontext.Context, machine bool) ([]byte, error) {
	location := c.WorktreePath
	if location == "" {
		location = c.WorkDir
	}
	links := c.State.Links
	if links == nil {
		links = map[string]string{}
	}
	v := statusView{ID: c.State.Work.ID, Status: c.State.Work.Status, Branch: c.State.Work.Branch, Location: location, Links: links}
	if machine {
		b, err := json.Marshal(v)
		return append(b, '\n'), err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "work: id %s\nwork: status %s\nwork: branch %s\nwork: path %s\n", statusText(v.ID), statusText(v.Status), statusText(v.Branch), statusText(v.Location))
	keys := make([]string, 0, len(v.Links))
	for key := range v.Links {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		out.WriteString("work: links none\n")
	}
	for _, key := range keys {
		fmt.Fprintf(&out, "work: link %s %s\n", statusText(key), statusText(v.Links[key]))
	}
	return out.Bytes(), nil
}

// statusText keeps each human record on one terminal-safe line without
// truncation. JSON serialization preserves the original saved text separately.
func statusText(value string) string {
	for _, r := range value {
		if !unicode.IsPrint(r) {
			return strconv.Quote(value)
		}
	}
	return value
}
