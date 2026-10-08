const { workspace, window, commands } = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

/** @type {LanguageClient | undefined} */
let client;

function serverOptions() {
  const config = workspace.getConfiguration("featx");
  const command = config.get("server.path") || "featx-lsp";
  const args = config.get("server.args") || [];

  // stdio is what featx-lsp speaks; it writes its own logs to stderr, which
  // the client surfaces in the output channel.
  const run = { command, args, transport: TransportKind.stdio };

  return { run, debug: run };
}

async function start() {
  client = new LanguageClient(
    "featx",
    "FeatX Language Server",
    serverOptions(),
    {
      documentSelector: [{ scheme: "file", language: "featx" }],
      synchronize: {
        // A model saved outside the editor changes the verdict on every open
        // configuration, so let the server know about any .ftx that changes.
        fileEvents: workspace.createFileSystemWatcher("**/*.ftx"),
      },
      outputChannelName: "FeatX",
    },
  );

  try {
    await client.start();
  } catch (err) {
    client = undefined;

    const path = workspace.getConfiguration("featx").get("server.path");
    window.showErrorMessage(
      `FeatX: could not start "${path}". Build it with ` +
        "`go build ./cmd/featx-lsp` and set featx.server.path to the binary, " +
        `or put it on your PATH. (${err.message})`,
    );
  }
}

async function stop() {
  if (!client) {
    return;
  }

  const stopping = client;
  client = undefined;
  await stopping.stop();
}

function activate(context) {
  context.subscriptions.push(
    commands.registerCommand("featx.restartServer", async () => {
      await stop();
      await start();
    }),
  );

  // Restart when the server path or arguments change, since neither can be
  // applied to a running process.
  context.subscriptions.push(
    workspace.onDidChangeConfiguration(async (event) => {
      if (
        event.affectsConfiguration("featx.server.path") ||
        event.affectsConfiguration("featx.server.args")
      ) {
        await stop();
        await start();
      }
    }),
  );

  return start();
}

function deactivate() {
  return stop();
}

module.exports = { activate, deactivate };
