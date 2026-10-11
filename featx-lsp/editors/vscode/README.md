# FeatX for VS Code

Language support for `.ftx` feature models and feature configurations: live
diagnostics, hover, completion, outline, formatting and go-to-definition from a
configuration entry to its declaration in the model.

This extension is only a thin client. All the analysis happens in `featx-lsp`,
which must be built first.

## Install

**1. Build the server and put it on your `PATH`:**

```bash
cd featx-lsp
go build -o featx-lsp ./cmd/featx-lsp
sudo install featx-lsp /usr/local/bin/
```

If you would rather not install it system-wide, leave the binary where it is
and set `featx.server.path` to its absolute path in VS Code settings.

**2. Install the extension's one dependency:**

```bash
cd featx-lsp/editors/vscode
npm install
```

**3. Package and install it:**

```bash
npx @vscode/vsce package --allow-missing-repository
code --install-extension featx-0.1.0.vsix --force
```

Then reload VS Code: Command Palette (`Ctrl+Shift+P`) → **Developer: Reload
Window**.

Confirm it took with `code --list-extensions | grep featx`, which should print
`isselab.featx`.

> Do **not** try to install this by copying or symlinking the directory into
> `~/.vscode/extensions/`. Current VS Code versions only load extensions
> recorded in `~/.vscode/extensions/extensions.json`, and that file is written
> by `code --install-extension`. A stray directory there is ignored silently,
> with no error to explain why nothing happened.

To hack on the extension instead, open this directory in VS Code and press
`F5`, which launches a second window with the extension loaded and needs no
installation at all. From a shell the equivalent is:

```bash
code --extensionDevelopmentPath="$PWD" /path/to/your/ros2_ws
```

After changing `extension.js`, reload the window. After rebuilding the Go
server, reinstall it (`sudo install featx-lsp /usr/local/bin/`) and run
**FeatX: Restart Language Server** — a reload is not needed for server-only
changes.

## Check it works

Open `featxcli/featxcli/model/configs.ftx`. You should get:

- the outline in the Explorer's **Outline** pane
- hover over a feature name showing its binding and its model declaration
- `F12` on a feature name jumping to `features.ftx`
- completion offering `static`/`dynamic`/`early`/`late` after the colon, and
  unconfigured feature names at the start of a line

To confirm the diagnostics are live, change a binding to one the model does not
allow — `navigation *: dynamic early` — and the line should be marked.

If nothing happens, run **FeatX: Restart Language Server** from the command
palette, then check the **FeatX** output channel. Setting
`featx.trace.server` to `verbose` logs the whole LSP conversation there.

## Settings

| Setting | Default | Meaning |
|---|---|---|
| `featx.server.path` | `featx-lsp` | Path to the server binary; a bare name is looked up on `PATH`. |
| `featx.server.args` | `[]` | Extra arguments for the server. |
| `featx.trace.server` | `off` | Log LSP traffic to the FeatX output channel. |

Changing the path or arguments restarts the server automatically.

## Without VS Code

The same checks run from a terminal, which is also the form to use in CI or a
pre-commit hook:

```bash
featx-lsp -lint featxcli/featxcli/model
```

It prints `path:line:col: severity: message` and exits non-zero if it found any
errors. Warnings do not fail it.
