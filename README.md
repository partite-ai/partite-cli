# partite

The command line tool for [Partite](https://partite.ai). It manages everything
the terraform provider manages, plus mesh versions, and it runs meshes:
conversations, streaming, tasks and traces.

It is built to be used by people at a terminal and by agents and scripts:

- Tables at a terminal, JSON otherwise. `-o json`, `-o yaml` and `-o id`
  choose a format.
- `get -o yaml` prints the same format that `-f` reads, so you can save a
  resource, edit it and `apply` it.
- Anywhere a command takes an id, it also takes a name.
- It never prompts without a terminal. Deletes need `--yes` there.
- Errors say what to do next, and exit codes mean one thing each (see below).

## Install

    go install github.com/partite-ai/partite-cli/cmd/partite@latest

## Log in

    partite login               # opens a login page in your browser
    partite login --no-browser  # over SSH: open the page on another device
    partite login --org org_x   # log in to a particular organization
    partite whoami

The tokens are kept in the OS keychain and refreshed automatically. Set
`PARTITE_NO_KEYRING=1` to keep them in `~/.config/partite/credentials.json`
(mode 0600) instead.

For scripts and CI, use an API key:

    partite api-key create ci --scope configuration:read --scope configuration:write
    export PARTITE_API_TOKEN=prt_ak_...

## Contexts

A context is a server and how to log in to it. Two are built in:

| Context | Server                   | Auth            |
|---------|--------------------------|-----------------|
| `cloud` | `https://api.partite.ai` | `partite login` |
| `local` | `http://localhost:8080`  | none            |

On cloud, `view` opens a resource in the web app, such as
`partite agent view triage` or `partite mesh view` for the list. It prints
the URL too. When you are logged in, or the context has `--org`, it opens
the web app's direct link, which switches the browser to the record's org
first. Another context can name its web app with
`partite context set NAME --web-url URL`.

`partite login` works only with Partite cloud. Other servers take an API key:

    partite context set acme --url https://partite.acme.example.com
    echo "$PARTITE_API_TOKEN" | partite --context acme login --with-api-key
    partite context use local

## A tour

    # Models
    partite model-provider create anthropic --type anthropic --api-key-env ANTHROPIC_API_KEY
    partite model-profile create sonnet --provider anthropic --model claude-sonnet-5-5
    partite model-set create default --default sonnet

    # An agent and a mesh
    partite agent create helper --task-instructions @instructions.md \
        --model-complexity medium --input-message-type @input.json
    partite mesh apply -f mesh.yaml          # the mesh with its inputs and links
    partite mesh publish support --label v1

    # Run it (a new conversation needs --label, --version, --route or --draft)
    partite run support --label v1 "My order is late"
    partite conversation intents rcnv_...    # what the conversation accepts
    partite chat support --label v1          # an interactive conversation
    partite trace get rcnv_...               # what happened, step by step
    partite trace view rcnv_...              # the same in the web app (cloud)

    # Move it to terraform (enterprise)
    partite export mesh support --all --import-blocks -O support.tf

Run `partite --help` for the full list, and `--help` on any command for its
flags and examples.

## For agents and scripts

- Pass `-o json`. Lists print arrays, and single resources print objects with
  the terraform provider's attribute names.
- `partite run MESH --label LABEL TEXT -o json` prints one result object:
  `{conversation_id, request_id, status, response | error | task}`.
  `--events` prints one JSON line per event and ends with the result.
- When a request waits for a task, `run` exits with 9 and prints the task as
  the server sent it: its input and the JSON schema of its answer. Answer it
  with `partite conversation task complete`, or decline it with `partite
  conversation task cancel`. At a terminal, `run` and `chat`
  ask for the answer instead, and draw the task's A2UI form when it has one
  from the basic catalog.
- `partite conversation intents CONVERSATION -o json` lists what a
  conversation accepts and the input schema of each intent. To see a mesh
  version's intents, create a conversation on it first with
  `partite conversation create MESH --label LABEL -o id`.
- A conversation is named by its id (`rcnv_...`) or by the external id it was
  created with.
- Errors go to stderr. With `-o json` they are JSON:
  `{"error": {"message", "code", "hint", "param"}}`. `param`, when present,
  names the input the server found the problem with.

### Exit codes

| Code | Meaning |
|------|---------|
| 0    | success |
| 1    | error |
| 2    | bad usage, or a confirmation is needed (pass `--yes`) |
| 3    | not logged in, or the login expired |
| 4    | permission denied |
| 5    | not found |
| 6    | conflict, such as a name that is taken |
| 7    | the mesh request failed |
| 8    | the mesh request was cancelled |
| 9    | the mesh request is waiting for a task |
| 10   | the mesh request was still running when `--timeout` passed |
| 130  | cancelled with Ctrl-C |

## Environment

| Variable | |
|----------|-|
| `PARTITE_CONTEXT` | context to use, like `--context` |
| `PARTITE_URL` | server URL, overriding the context's |
| `PARTITE_API_TOKEN` | an API key token; used before any stored login. The older `PARTITE_API_KEY` and `PARTITE_API_SECRET` still work |
| `PARTITE_TOKEN` | an OAuth access token |
| `PARTITE_CONFIG_DIR` | where `config.yaml` lives (default `~/.config/partite`) |
| `PARTITE_NO_KEYRING` | keep secrets in a file, not the OS keychain |

## How login works

Login uses Kinde's device authorization flow (RFC 8628) at
`https://auth.partite.ai`, with the CLI's Kinde client id built in. The CLI
opens the login page with a code filled in and waits for you to approve it. It
does not run a local web server.

## Target rubric metrics to agents

Use `partite eval rubric get RUBRIC -o yaml` to save a rubric, then edit and apply the file. Add `tags` to a judge or computed metric assignment:

```yaml
name: support-quality
metrics:
  - computed_metric_key: latency_ms
    tags:
      - name: role
        value: reviewer
      - name: role
        value: planner
```

A metric runs for an eligible agent if any tag matches both name and value exactly. Matching is case-sensitive. An empty list applies the metric without a tag restriction. Request-scoped judges ignore these tags.

For an existing metric, omit `tags` to keep its tags. Use `tags: []` or `tags: null` to clear them. Supply a list to replace them. Apply the file with `partite eval rubric apply -f rubric.yaml`.
## Development

    GOWORK=off go build ./... && GOWORK=off go test ./...
    scripts/generate.sh      # regenerate the GraphQL client; needs ../mesh and ../mesh-enterprise

Use `GOWORK=off` when a parent directory has a `go.work` file that does not
include this module. In PowerShell, set `$env:GOWORK = 'off'` before the Go commands.

### GitHub Actions builds

`.github/workflows/ci.yml` tests and builds these targets on native runners:

| OS | Architecture | Archive |
|----|--------------|---------|
| Linux | AMD64 | `partite-linux-amd64.tar.gz` |
| Linux | ARM64 | `partite-linux-arm64.tar.gz` |
| macOS | Intel AMD64 | `partite-darwin-amd64.tar.gz` |
| macOS | Apple Silicon ARM64 | `partite-darwin-arm64.tar.gz` |
| Windows | AMD64 | `partite-windows-amd64.zip` |
| Windows | ARM64 | `partite-windows-arm64.zip` |

The workflow runs on pushed tags that start with `v` and manual requests.
Branch pushes and pull requests do not trigger it.

Tag builds set `partite --version` to the tag, such as `v0.1.0`. Other builds
use `dev-` followed by the first 12 characters of the commit SHA.

### Release downloads

```text
https://github.com/partite-ai/cli/releases/latest/download/partite-linux-arm64.tar.gz
```

For a fixed version, use:

```text
https://github.com/partite-ai/cli/releases/download/v0.1.0/partite-linux-arm64.tar.gz
```

Replace the archive name with any entry in the build table. The same URL
formats work for `checksums.txt`. These URLs become available after a release
is published. Public repositories allow anonymous downloads. For a private
repository, use an authenticated GitHub CLI session:

```sh
gh release download --repo partite-ai/cli \
  --pattern partite-linux-arm64.tar.gz --pattern checksums.txt --dir ./download
```

The workflow does not create installers or sign binaries.

See [AGENTS.md](AGENTS.md) for the code layout and conventions.
