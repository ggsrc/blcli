# Agent Replay Fixtures

Offline replay scenarios for validating v2 Agent workflows without touching real infrastructure.

These scenarios are safe because they use static logs and read-only contract/run inspection commands. They do not execute Terraform, kubectl, gcloud, gh, or cloud APIs.

## Scenario: classify a known failure

```bash
blcli contract --format json
blcli diagnose --file integration/fixtures/failures/resource_already_exists.log --format json
blcli diagnose --file integration/fixtures/failures/state_lock_conflict.log --format json
blcli diagnose --file integration/fixtures/failures/credential_invalid.log --format json
```

Expected Agent behavior:

- Reads the tool contract before choosing commands.
- Uses `diagnose --format json` for machine-readable failure classification.
- Treats `repair_commands` as candidates, not as automatically approved actions.
- Requires human approval before any destructive or production-impacting repair.

## Scenario: inspect recorded runs

```bash
blcli runs list --format json
blcli runs list --status failed --format json
blcli runs show <operation-id> --format json
```

Expected Agent behavior:

- Uses `runs list` to discover operation ids.
- Uses `runs show` to inspect step `status`, `command`, `output_excerpt`, `error_location`, and `error_message`.
- Runs `diagnose` against the first failed step output when a known failure needs classification.
