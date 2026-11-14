# role-matrix

Generated from validator/protocol descriptors. Do not edit.

<!-- fixture:descriptor -->
```json
{
  "Identity": {
    "Grammar": "^[A-Za-z0-9][A-Za-z0-9._-]*$",
    "MaxBytes": 64,
    "Reserved": "builtin",
    "Devices": [
      "aux",
      "com1",
      "com2",
      "com3",
      "com4",
      "com5",
      "com6",
      "com7",
      "com8",
      "com9",
      "con",
      "lpt1",
      "lpt2",
      "lpt3",
      "lpt4",
      "lpt5",
      "lpt6",
      "lpt7",
      "lpt8",
      "lpt9",
      "nul",
      "prn"
    ]
  },
  "Semconv": {
    "Facts": [
      "base_branch",
      "branch",
      "slug",
      "start_mode",
      "worktree_path"
    ],
    "PublicGrammar": "^[a-z][a-z0-9_]*(\\.[a-z][a-z0-9_]*)+$",
    "LocalGrammar": "^[a-z][a-z0-9_]*(\\.[a-z][a-z0-9_]*)*$",
    "PrivatePrefix": "plugin."
  },
  "IPC": {
    "Types": {
      "ImporterInput": [
        {
          "Name": "inputs",
          "Type": "map[string]interface {}"
        },
        {
          "Name": "output_dir",
          "Type": "string"
        }
      ],
      "LinkerInput": [
        {
          "Name": "inputs",
          "Type": "map[string]interface {}"
        }
      ],
      "LinkerResponse": [
        {
          "Name": "value",
          "Type": "string"
        }
      ],
      "LocatorInput": [
        {
          "Name": "repository",
          "Type": "ipc.RepositoryReference"
        },
        {
          "Name": "repository_roots",
          "Type": "[]string"
        }
      ],
      "LocatorResponse": [
        {
          "Name": "matches",
          "Type": "[]ipc.Match"
        }
      ],
      "Match": [
        {
          "Name": "repo_path",
          "Type": "string"
        }
      ],
      "RepositoryReference": [
        {
          "Name": "git_fetch_urls,omitempty",
          "Type": "[]string"
        },
        {
          "Name": "name,omitempty",
          "Type": "string"
        },
        {
          "Name": "path,omitempty",
          "Type": "string"
        },
        {
          "Name": "query,omitempty",
          "Type": "string"
        }
      ],
      "StarterInput": [
        {
          "Name": "arg",
          "Type": "string"
        }
      ],
      "StarterResponse": [
        {
          "Name": "base_branch,omitempty",
          "Type": "string"
        },
        {
          "Name": "links,omitempty",
          "Type": "map[string]string"
        },
        {
          "Name": "meta,omitempty",
          "Type": "map[string]interface {}"
        },
        {
          "Name": "modes,omitempty",
          "Type": "json.RawMessage"
        },
        {
          "Name": "repository",
          "Type": "ipc.RepositoryReference"
        },
        {
          "Name": "start_modes,omitempty",
          "Type": "[]string"
        }
      ]
    },
    "StdoutLimit": 1048576,
    "StderrTail": 4096
  }
}
```
