# Failure Fixtures

Offline failure samples for validating `blcli diagnose` and Agent repair flows.

These fixtures are safe to replay because they are static logs. They do not execute Terraform, kubectl, gcloud, gh, or any cloud operation.

## Replay

```bash
blcli diagnose --file integration/fixtures/failures/resource_already_exists.log --format json
blcli diagnose --file integration/fixtures/failures/state_lock_conflict.log --format json
blcli diagnose --file integration/fixtures/failures/credential_invalid.log --format json
```

Expected categories are listed in `metadata.yaml`.
