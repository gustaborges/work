package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	skills "github.com/gustaborges/work/ai-skills"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/skillinstall"
	"github.com/spf13/cobra"
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
		var vendors []string
		var yes, force bool
		cmd := &cobra.Command{Use: operation, Short: operation + " the complete bundled Work skill collection", Args: noArgsUsage}
		cmd.Flags().StringVar(&o.Scope, "scope", "", "required: local (current directory) or user")
		cmd.Flags().StringArrayVar(&vendors, "vendor", nil, "required: exactly one of claude or codex")
		if operation == "install" {
			cmd.Flags().BoolVar(&force, "force", false, "replace ordinary modified or foreign content")
			cmd.Flags().BoolVar(&yes, "yes", false, "skip the installation confirmation (does not permit replacing local edits)")
		} else {
			cmd.Flags().Bool("json", false, "emit machine-readable status")
		}
		cmd.RunE = func(c *cobra.Command, args []string) error {
			machine, _ := c.Flags().GetBool("json")
			if operation == "install" && c.Flags().Changed("json") {
				return diag.New(diag.Usage, "skills install does not accept --json")
			}
			if len(vendors) != 1 {
				return diag.New(diag.Usage, "--vendor is required and must select exactly one of claude or codex")
			}
			o.Vendor = vendors[0]
			ds, err := skillinstall.Resolve(o)
			if err != nil {
				return diag.Wrap(diag.Usage, err, "choose --scope=local or --scope=user and exactly one --vendor=claude or --vendor=codex")
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
			if err := confirmSkillsInstall(c, ds, o, yes, force); err != nil {
				return err
			}
			outcomes := skillinstall.Install(c.Context(), ds, force)
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
						fmt.Fprintf(c.ErrOrStderr(), "hint: invoke /%s in Claude Code; use /reload-skills if the top-level skills directory was created during this session\n", out.Destination.Skill)
					} else {
						fmt.Fprintf(c.ErrOrStderr(), "hint: invoke $%s in Codex; restart if it is not detected\n", out.Destination.Skill)
					}
				}
			}
			return failure
		}
		group.AddCommand(cmd)
	}
	return group
}

func skillsInstallImpact(ds []skillinstall.Destination, o skillinstall.Options, force bool) string {
	var impact strings.Builder
	fmt.Fprintf(&impact, "Vendor: %s\nScope: %s\n\nSkills included:\n", o.Vendor, o.Scope)
	for _, d := range ds {
		fmt.Fprintf(&impact, "  %s\n    → %q\n", d.Skill, d.Path)
	}
	if force {
		impact.WriteString("\nReplace modified or foreign skill files if present. Local edits will be lost.\n")
	}
	return impact.String()
}

func confirmSkillsInstall(c *cobra.Command, ds []skillinstall.Destination, o skillinstall.Options, yes, force bool) error {
	if yes {
		return nil
	}
	if !present.IsInteractive() {
		return diag.New(diag.Usage, "skills install requires --yes in a non-interactive session")
	}
	ok, err := present.Confirm(c.Context(), present.IO{In: c.InOrStdin(), UI: c.ErrOrStderr()}, present.ConfirmSpec{
		Title: "Install Work skills", Impact: skillsInstallImpact(ds, o, force), Accept: "Install", Reject: "Cancel",
	})
	if err != nil {
		return err
	}
	if !ok {
		return diag.New(diag.Cancelled, "skill installation declined; no files changed")
	}
	return nil
}
