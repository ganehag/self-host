# selfctl

`selfctl` is the operator CLI for Self-host.

It supports:
- API-backed resource management for `dataset`, `user`, `group`, `policy`, `thing`, and `timeseries`
- local config management through `selfctl config`
- raw API escape hatches through `selfctl api request`
- local program tooling and DB tooling that already existed in the command

## Config

By default, `selfctl` reads:

```text
$HOME/.selfctl/config.yaml
```

Minimal example:

```yaml
api:
  server: http://127.0.0.1:8080
  domain: test
  token: root
```

You can manage this with:

```bash
selfctl config init --server http://127.0.0.1:8080 --domain test --token root
selfctl config view
selfctl config set api.server http://127.0.0.1:18080
selfctl config unset api.token
```

Global API override flags are available on all API-backed commands:

```bash
selfctl --server http://127.0.0.1:18080 --domain test --token root dataset list
```

Environment overrides also work:

```bash
export SELFCTL_API_SERVER=http://127.0.0.1:8080
export SELFCTL_API_DOMAIN=test
export SELFCTL_API_TOKEN=root
```

## Common Commands

Datasets:

```bash
selfctl dataset list
selfctl dataset get <dataset-uuid>
selfctl dataset create big-upload-test --dataset-format bin
selfctl dataset update <dataset-uuid> --name renamed --tags a,b
selfctl dataset upload ./big.bin --name big-upload-test
selfctl dataset download <dataset-uuid> -o out.bin
selfctl dataset delete <dataset-uuid>
```

Users, groups, and policies:

```bash
selfctl user whoami
selfctl user list
selfctl user create alice
selfctl user token create <user-uuid> "CLI token"

selfctl group list
selfctl group create operators

selfctl policy list
selfctl policy create --group-uuid <group-uuid> --effect allow --action read --resource timeseries/%
selfctl policy explain read timeseries/123/data
```

Things and time series:

```bash
selfctl thing create boiler-1 --thing-type boiler --tags plant-a,heat
selfctl thing update <thing-uuid> --state active
selfctl thing datasets <thing-uuid>
selfctl thing timeseries <thing-uuid>

selfctl timeseries create temperature --si-unit C --thing-uuid <thing-uuid>
selfctl timeseries update <ts-uuid> --upper-bound 100
```

Raw API requests:

```bash
selfctl api request GET /v2/users/me
selfctl api request POST /v2/groups --body '{"name":"operators"}'
```

## Output

Most list/get commands default to terminal-friendly output and support:

```bash
--format table
--format json
```

Dataset list/get commands also support:

```bash
--size-format human
--size-format bytes
```

## Shell Completion

`selfctl` uses Cobra completion and also provides custom completions for:
- common enum flags such as `--format`, `--size-format`, `--effect`, `--action`, and `--state`
- config keys for `selfctl config set` and `selfctl config unset`
- common UUID arguments by querying the configured API when possible

Generate completion scripts from the current binary:

```bash
selfctl completion bash > ~/.local/share/bash-completion/completions/selfctl
```

```bash
mkdir -p ~/.zsh/completions
selfctl completion zsh > ~/.zsh/completions/_selfctl
```

Reload your shell after updating the script.
