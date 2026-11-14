package cli

import (
	"encoding/json"
	"fmt"
	skills "github.com/gustaborges/work/ai-skills"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/skillinstall"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

func skillError(token string) error {
	for _, c := range diag.All {
		if c.Token == token {
			return diag.New(c, "skill operation needs attention; inspect the selected destination and retry explicitly")
		}
	}
	return diag.New(diag.SkillIOFailed, "skill operation failed")
}
func newSkillsCmd() *cobra.Command {
	group := &cobra.Command{Use: "skills", Short: "Install and inspect bundled agent authoring skills", Args: noArgsUsage, RunE: func(c *cobra.Command, _ []string) error { return c.Help() }}
	for _, operation := range []string{"install", "status"} {
		var o skillinstall.Options
		cmd := &cobra.Command{Use: operation + " [SKILL...]", Short: operation + " local agent skill copies"}
		cmd.Flags().BoolVar(&o.Global, "global", false, "use personal agent skill directories")
		cmd.Flags().StringVar(&o.Dir, "dir", "", "use an existing folder")
		cmd.Flags().StringArrayVar(&o.Vendors, "vendor", nil, "select claude or codex (repeatable; default both)")
		if operation == "install" {
			cmd.Flags().BoolVar(&o.Force, "force", false, "replace ordinary modified or foreign content")
		} else {
			cmd.Flags().Bool("json", false, "emit machine-readable status")
		}
		cmd.RunE = func(c *cobra.Command, args []string) error {
			machine, _ := c.Flags().GetBool("json")
			if operation == "install" && c.Flags().Changed("json") {
				return diag.New(diag.Usage, "skills install does not accept --json")
			}
			if c.Flags().Changed("dir") && o.Dir == "" {
				return diag.New(diag.Usage, "--dir must be nonempty")
			}
			o.Skills = args
			ds, err := skillinstall.Resolve(o)
			if err != nil {
				return diag.Wrap(diag.Usage, err, "invalid skill selection or scope; choose --global or an existing --dir and supported skill/vendor names")
			}
			if o.Dir != "" {
				if _, err := os.Stat(filepath.Join(o.Dir, "marketplace.json")); os.IsNotExist(err) {
					fmt.Fprintf(c.ErrOrStderr(), "note: no marketplace.json in %q; skill management does not require one\n", o.Dir)
				}
			}
			if operation == "status" {
				rows := make([]skillinstall.Result, 0, len(ds))
				var failure error
				rank := 0
				for _, d := range ds {
					digest, err := skills.Digest(d.Skill)
					if err != nil {
						return skillError("skill-io-failed")
					}
					r := skillinstall.Inspect(d, digest)
					rows = append(rows, r)
					priority := 0
					switch r.Error {
					case "skill-io-failed":
						priority = 1
					case "skill-target-conflict":
						priority = 2
					case "skill-recovery-required":
						priority = 3
					}
					if priority > rank {
						rank = priority
						failure = skillError(r.Error)
					}
				}
				if machine {
					if err := json.NewEncoder(c.OutOrStdout()).Encode(rows); err != nil {
						return err
					}
				} else {
					for _, r := range rows {
						fmt.Fprintf(c.OutOrStdout(), "%s vendor=%s skill=%s path=%q", r.State, r.Vendor, r.Skill, r.Path)
						if r.Error != "" {
							fmt.Fprintf(c.OutOrStdout(), " error=%s", r.Error)
						}
						fmt.Fprintln(c.OutOrStdout())
					}
				}
				return failure
			}
			outcomes := skillinstall.Install(c.Context(), ds, o.Force)
			var failure error
			for _, out := range outcomes {
				fmt.Fprint(c.OutOrStdout(), out.String())
				if out.Error != "" {
					fmt.Fprintf(c.OutOrStdout(), " error=%s", out.Error)
					if failure == nil {
						failure = skillError(out.Error)
					}
				}
				fmt.Fprintln(c.OutOrStdout())
				if out.Error == "" && (out.Action == "installed" || out.Action == "updated" || out.Action == "unchanged") {
					if out.Destination.Vendor == "claude" {
						fmt.Fprintln(c.ErrOrStderr(), "hint: invoke /work-plugin-builder in Claude Code; use /reload-skills if the top-level skills directory was created during this session")
					} else {
						fmt.Fprintln(c.ErrOrStderr(), "hint: invoke $work-plugin-builder in Codex; restart if it is not detected")
					}
				}
			}
			return failure
		}
		group.AddCommand(cmd)
	}
	return group
}
