# manifest-schema

Generated from validator/protocol descriptors. Do not edit.

<!-- fixture:descriptor -->
```json
{
  "TopLevel": [
    "components",
    "conventions",
    "name",
    "version"
  ],
  "Component": [
    "accepts",
    "description",
    "discover",
    "display_name",
    "entrypoint",
    "inputs",
    "key",
    "manual",
    "name",
    "on",
    "pattern",
    "role",
    "runtime"
  ],
  "Convention": [
    "name",
    "prefixes"
  ],
  "Roles": [
    "importer",
    "linker",
    "repository-locator",
    "starter"
  ],
  "Accepts": [
    "git_fetch_urls",
    "name",
    "query"
  ],
  "Events": [
    "start:finalized"
  ],
  "InputsGrammar": "^(work|meta|link):([^:]+)(:optional)?$",
  "Rules": {
    "importer": {
      "Required": [
        "entrypoint",
        "name",
        "role"
      ],
      "Allowed": [
        "inputs",
        "manual",
        "on",
        "runtime"
      ],
      "Forbidden": [
        "accepts",
        "description",
        "discover",
        "display_name",
        "key",
        "pattern"
      ],
      "AnyOf": [
        [
          "manual",
          "on"
        ]
      ]
    },
    "linker": {
      "Required": [
        "entrypoint",
        "key",
        "name",
        "role"
      ],
      "Allowed": [
        "discover",
        "manual",
        "runtime"
      ],
      "Forbidden": [
        "accepts",
        "description",
        "display_name",
        "inputs",
        "on",
        "pattern"
      ],
      "AnyOf": [
        [
          "discover",
          "manual"
        ]
      ]
    },
    "repository-locator": {
      "Required": [
        "accepts",
        "entrypoint",
        "name",
        "role"
      ],
      "Allowed": [
        "description",
        "display_name",
        "runtime"
      ],
      "Forbidden": [
        "discover",
        "inputs",
        "key",
        "manual",
        "on",
        "pattern"
      ],
      "AnyOf": []
    },
    "starter": {
      "Required": [
        "entrypoint",
        "name",
        "role"
      ],
      "Allowed": [
        "pattern",
        "runtime"
      ],
      "Forbidden": [
        "accepts",
        "description",
        "discover",
        "display_name",
        "inputs",
        "key",
        "manual",
        "on"
      ],
      "AnyOf": []
    }
  }
}
```
