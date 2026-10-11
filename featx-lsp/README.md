# featx-lsp

A Language Server Protocol implementation for the FeatX DSL — the `.ftx` feature
models and feature configurations under `featxcli/featxcli/model/`.

It gives an editor live diagnostics, hover, completion, an outline, formatting
and go-to-definition for `.ftx` files, and validates a configuration against
its model while you type. It covers the rules `featxcli`'s
`Configurator.checkRules` enforces at the command line, and a good deal besides
— types, parameter values and value constraints — all reported before you run
anything.

No third-party dependencies: the JSON-RPC framing and the LSP message types are
in this module, so `go build` works offline.

## Build

```
cd featx-lsp && go build -o featx-lsp ./cmd/featx-lsp
```

To install it on your `PATH`:

```
go install github.com/isselab/featxsuite/featx-lsp/cmd/featx-lsp@latest
```

## Running it

The binary has four modes.

**As a language server**, which is how an editor uses it. It speaks LSP over
stdin and stdout, so running it from a shell looks like it has hung — it is
waiting for a `Content-Length` frame. Editors launch it themselves; see
[editor setup](#editor-setup). Logs go to stderr; `-quiet` discards them. A
`-stdio` flag is accepted and ignored, because LSP clients append it when they
launch a server and Go's flag package treats an unknown flag as fatal.

**As a one-shot checker**, with `-lint`, for a terminal, a pre-commit hook or
CI. It takes files or directories, walking directories for `.ftx` files:

```
$ featx-lsp -lint featxcli/featxcli/model
checked 2 file(s): 0 error(s), 0 warning(s)
```

Output is `path:line:col: severity: message`, which editors and CI log parsers
already understand. The exit status is 0 when clean, 1 when any **error** was
found, and 2 when the arguments themselves were wrong. Warnings and hints do
not fail it, so unfinished placeholders do not break a build.

Every mode runs the same checks through `internal/lint`, so the editor, the
terminal and the two generators can never disagree about whether a document is
valid.

**As an implementation check**, with `-lint -implementations`, which asks
whether each node in the model actually has something implementing it in the
workspace you have built:

```
$ featx-lsp -lint -implementations featxcli/featxcli/model
features.ftx:5:9: error: no component is registered as "Lifter" for feature "lifter"; register it with rclcpp_components_register_nodes, or for rclpy a line in the package's components file
features.ftx:6:5: error: feature "gripper" is bound static and early, so package "gripper" must install an executable "gripper" composing it; nothing does
```

What must exist depends on the bindings, so the check does too. A feature bound
`static early` is compiled into a manual composition and launched, so it needs
an executable named after it; registering a component of the same name does not
make it bindable. Everything else is loaded, so it needs a registered component
whose class name is the feature's name in `PascalCase` — the only link between
a feature and its implementation, and the easiest thing to get wrong. A typed
feature is a parameter of the node that encloses it and is not checked, having
nothing to implement.

It also rejects a `static early` feature implemented in rclpy. That binding
leaves a deselected feature out of the binary, which is the preprocessor's
doing, and there is no compile step in Python to leave anything out of. The
check looks at which group registered the class, and reports the feature before
looking for an executable — because an `ament_python` console script named
after its package installs exactly where a manual composition is looked for, so
an executable existing proves nothing here:

```
features.ftx:2:5: error: feature "gripper" is bound static and early, but "gripper" implements it as an rclpy component; compile-time binding needs a compile step, so a deselected feature would stay installed. Bind it static late, or implement it in rclcpp
```

This is **not** one of the model's own diagnostics and the editor never reports
it. The other checks are decided by the `.ftx` files and hold wherever they are
read; this one depends on what happens to be built and sourced, so it would
show a sound model as broken until the workspace was built, and broken again
whenever you worked on a different one. For the same reason it refuses to guess
when `AMENT_PREFIX_PATH` is empty, rather than reporting every feature as
unimplemented:

```
$ featx-lsp -lint -implementations featxcli/featxcli/model
featx-lsp -lint -implementations: nothing is installed on AMENT_PREFIX_PATH, so
there is nothing to check against. Build the workspace and source its
install/setup.bash.
```

**As a parameter-schema generator**, with `-params`, which renders a model's
typed parameters as [generate_parameter_library](https://github.com/PickNikRobotics/generate_parameter_library)
specifications — one file per node that declares any:

```
$ featx-lsp -params -w featxcli/featxcli/model/features.ftx
wrote featxcli/featxcli/model/to_loading_params.yaml
```

The DSL's type names and validators were chosen to match the library's, so the
mapping is direct. Two fields are derived rather than written: `description` is
always blank, for a human to fill in, and `read_only` states the **binding
mode** of the node the specification is for: `true` for `static`, since such a
node cannot be rebound and so neither can its parameters, and `false` for
`dynamic`, whose parameters stay reconfigurable. Both are written out rather
than leaving the false case to the generator's default, so a specification
states what the model says instead of leaving it to be read from an absence.

A parameter written at a **grouping** is declared on each of its alternatives
rather than on the grouping itself. A grouping is a choice point: its children
are the alternatives, and one of them is what runs. Since a ROS parameter
belongs to a single node and the alternatives are nodes of their own, declaring
a maximum velocity on the grouping would put it on the node that does not move,
where the ones that do cannot read it. So this

```
xor navigation : node {static early} ?
    vel_max : double (1.6) bounds<0.0, 3.0>
    to_loading : node {dynamic late}
    to_delivery : node {dynamic late}
```

emits `to_loading_params.yaml` and `to_delivery_params.yaml`, each declaring
`vel_max`, and nothing for `navigation`. This is why `read_only` is read off
the receiving node rather than off the parameter's place in the model: these
two are bound dynamically, so their copies stay reconfigurable even though the
grouping above them is static. A plain node is a worker rather than a choice
point, so it keeps what it declares, and a grouping with no alternatives keeps
its parameters rather than dropping them silently.

**As a parameter-schema installer**, with `-sync`, which puts each
specification in the package that implements the node it belongs to, so a
parameter is declared once in the feature model and read from there. Point
`-into` at the directory holding the feature packages. Like the generators it
reports and changes nothing until given `-w`:

```
$ featx-lsp -sync -into src/packages featxcli/featxcli/model/features.ftx
  to_base        would create   src/packages/navigation/config/to_base_params.yaml
  to_delivery    would create   src/packages/navigation/config/to_delivery_params.yaml
  to_loading     would create   src/packages/navigation/config/to_loading_params.yaml

to_loading does not build its specification yet. Add to src/packages/navigation:
  package.xml:     <depend>generate_parameter_library</depend>
  CMakeLists.txt:  find_package(generate_parameter_library REQUIRED)
                   generate_parameter_library(to_loading_parameters config/to_loading_params.yaml)
                   target_link_libraries(to_loading to_loading_parameters)
```

The three specifications land in `navigation` because `to_loading` and its
siblings are components built by that package: the node is looked for first,
then the features it sits under. The target is named rather than left blank,
since a node is built as a target of its own name.

Each node is reported as `create`, `update` or `unchanged`, so re-running is
cheap and a hand-edited specification is restored to what the model says.

A specification the model no longer asks for is **removed**, which keeps a
parameter that moves — between nodes, or between packages — from leaving its
old file behind for a build to keep generating from. Every file written carries
a marker:

```yaml
# Generated by featx-lsp from the feature model.
# Edits are overwritten: change the .ftx model instead.
to_loading:
  vel_max:
```

Only a file carrying it is ever removed, so parameter schemas a package keeps
by hand are left alone even when their names look the same.
`generate_parameter_library` ignores comments, so the marker costs nothing.
Pruning looks through every package under `-into` rather than only the ones
being written to, since a parameter that changes package leaves its old file
somewhere this run never visits.

Two things are reported rather than done:

- **A node with no package of its own** exits non-zero and names the node, so a
  build fails instead of quietly dropping parameters that have nowhere to go.
  The node is tried first and then the features it sits under, since an
  alternative is usually a component inside the package built for the grouping
  that holds it — `to_loading`'s specification belongs to `navigation`.
- **A destination that does not build what it was given** is told what to add.
  Which target links the generated library is the package's business, and the
  advice follows the destination's build system — `generate_parameter_library`
  for CMake, `generate_parameter_module` for setuptools. The advice stops once
  the hook is there.

**As a JSON generator**, with `-emit`, so that the DSL is the single source of
truth and `features.json`/`configs.json` become build artifacts rather than
peers that can drift. Following `gofmt`, it prints to stdout and needs `-w` to
write:

```
$ featx-lsp -emit features.ftx            # render to stdout
$ featx-lsp -emit -w featxcli/featxcli/model
wrote featxcli/featxcli/model/features.json
wrote featxcli/featxcli/model/configs.json
```

The output reproduces the shape the existing readers expect, including the
capitalised binding vocabulary (`Early`, `Static`) and the empty-string rather
than empty-array convention for unset clauses that
`Configurator.find_feature_includes` branches on. A document with errors is
refused rather than rendered, so a half-typed model can never overwrite a
working JSON file.

Several fields differ from the hand-written JSON by design. `isOptional`
follows the `?` markers in the DSL, and `grouping`, `type`, `default`,
`requiresOperator` and `modelName` are added to record what the hand-written
format had no place for. No existing reader consumes any of them, and the outer
object keeps the name `"root"` because `Configurator.traverseModel` tests for
that exact string.

## The grammar it implements

Neither document kind carries a header, and the two have **different entry
grammars**. A model opens with its root feature and declares what each feature
is and what it allows; a configuration is a bare list of entries that name a
feature, mark it and fix its binding.

```
model       := root (modelEntry | constraint)*
root        := NAME ":"
modelEntry  := INDENT [grouping] NAME ":" type [bindings] [value] [validator] [marker]
constraint  := INDENT ["!"] "[" NAME (op NAME)* "]"

config      := configEntry*
configEntry := NAME [marker] [bindings] [value]

grouping   := "or" | "xor"
marker     := "?"                            -- optional   (models)
            | "*"                            -- selected   (configurations)
type       := "node" | "bool" | "int" | "double" | "string"
            | "bool_array" | "int_array" | "double_array" | "string_array"
bindings   := "{" mode time "}"
value      := "(" literal ("," literal)* ")"
validator  := NAME "<" [arg ("," arg)*] ">"
              -- bounds, gt, gt_eq, lt, lt_eq, one_of,
              -- fixed_size, size_gt, size_lt, not_empty
mode       := "static" | "dynamic"
time       := "early" | "late"
op         := "&&" | "||"
```

The parts after a model entry's type — bindings, value, validator, marker —
are each optional and may be written in any order; the ordering above is the
canonical one the formatter produces.

```
RB-1-BASE:                                      navigation *{static early}
    xor navigation : node {static early} ?      vel_max *
        vel_max : double (1.6)                  to_loading {dynamic late}
        to_loading : node {dynamic late}
            [square && attach && lift]
```

The root feature is the document, not a level of nesting: it takes no type,
no binding and no configuration entry, and the coverage hint does not ask for
one. Everything else hangs beneath it, four columns per level.

With no header to go by, the **root's colon** is what tells the two dialects
apart: a first line of `NAME:` and nothing else opens a model, and anything
else opens a configuration.

A configuration restates neither the type nor the grouping, and carries no
constraints — all three belong to the model. `*` marks a feature selected.

Whether an entry carries a binding follows from the type the model gave it, and
since a configuration declares no types the parser cannot tell: a node must be
configured with one, a parameter must not, and both are reported by the
cross-document check rather than the parser. A parameter may carry a value to
override the model's default — `vel_max *(2.0)`.

### Groupings

A grouping constrains which of a feature's **children** may be selected
together:

- `xor` — only one child may be selected at a time.
- `or` — more than one may be, with no upper bound.

A grouping therefore only means something on a node, which is the only thing
that can have children. A parameter of any type carrying one is an error.

### Selection reaches parameters

A typed parameter is part of the node it belongs to, so selecting the node
selects the parameter too: if a non-node child of a selected feature is not
itself selected, that is an error. Two consequences follow:

- Every parameter of a selected node needs a selected entry in the
  configuration, and the coverage hint asks for one.
- An `xor` group ranges over its **node** children only. A parameter's
  selection is forced rather than chosen, so counting it as an alternative
  would make every group unsatisfiable the moment the group was selected.

### Types

Every feature declares a type. A `node` is a bindable unit of behaviour and
must declare its bindings in braces. The rest are typed parameters of the node
that encloses them, and may carry a value in parentheses:

```
xor navigation : node {static early} ?
    vel_max : double (1.6) bounds<0.0, 3.0>
    joints : string_array (j1, j2)
```

The parameter types are `bool`, `int`, `double`, `string` and the four `_array`
forms — the same names `generate_parameter_library` uses, so no translation is
needed when generating its schema. An array's value is a comma-separated list;
within one element the tokens rejoin, so a string needs no quoting even when it
contains spaces: `label : string (go fast)`.

A parameter with no bindings of its own is bound with its enclosing node, which
hover reports. Giving a node a value, or a parameter children, is an error.

### Validators

A parameter may constrain its own value. Each validator is written by the name
`generate_parameter_library` gives it, with its arguments after it, so the
model and the generated schema use the same word:

| Written | Generated | Meaning |
|---|---|---|
| `bounds<0.0, 3.0>` | `bounds<>: [0.0, 3.0]` | within a range, both ends admitted |
| `gt<0.0>` | `gt<>: [0.0]` | greater than |
| `gt_eq<0.0>` | `gt_eq<>: [0.0]` | greater than or equal to |
| `lt<3.0>` | `lt<>: [3.0]` | less than |
| `lt_eq<3.0>` | `lt_eq<>: [3.0]` | less than or equal to |
| `one_of<slow, fast>` | `one_of<>: [["slow", "fast"]]` | one of these |
| `fixed_size<4>` | `fixed_size<>: [4]` | exactly this long |
| `size_gt<2>` | `size_gt<>: [2]` | longer than |
| `size_lt<20>` | `size_lt<>: [20]` | shorter than |
| `not_empty<>` | `not_empty<>: []` | not of zero length |

The first five need a numeric type. `one_of` takes any parameter. The last four
measure a **length**, which a `string` and an array both have, so
`fixed_size<8>` is eight characters on a string and eight elements on an array.

`bounds<>` must stand alone. It gives both ends at once, so a comparison or an
enumeration written beside it states a limit the parameter already has, either
repeating one end or disagreeing with it. A range open at one end is therefore
written as the pair it is — `gt<0.0> lt_eq<3.0>` — rather than as `bounds<>`
with a correction after it.

A parameter may otherwise carry **several** validators. The library applies
each in turn, so a length range is a pair of them:

```
mode : string ("autonomous") not_empty<> size_lt<20>
```

Each may instead sit on its own line beneath the parameter, the way a
constraint sits beneath its feature. Which you choose is kept — the formatter
moves neither onto the other:

```
mode : string ("autonomous")
    one_of<"autonomous", "manual">
    size_lt<20>
```

Naming them replaced an older notation that inferred the validator from the
shape of its brackets — `<0.0, 3.0>` for a range, `<4>` for a size, `#` for an
unbounded end, `!` for an excluded one, `<<a, b>>` for an enumeration. The
shape had to carry the meaning because there was nowhere to put a name, and it
had run out of room: `not_empty` takes no arguments, which no arrangement of
brackets can express. Those forms are rejected with a message naming what to
write instead.

Beyond the type, the checker reports an unknown validator with the nearest
match, the wrong number of arguments, a validator twice on one parameter, a
pair that nothing can satisfy (`gt<10.0> lt<5.0>`), a comparison written beside
`bounds<>`, a range whose ends are reversed, and a default its own validators
reject — including one too short or
too long for a size. All of these would otherwise wait until the node started.

### Constraints

Containment is carried by nesting, so the cross-tree constraints sit on their
own bracketed line one level in from the entry they govern:

```
to_loading : node {dynamic late}
    [square && attach && lift]
```

`[...]` requires, `![...]` excludes. `&&` needs every operand and `||` any one
of them. Mixing the two connectives in one constraint is rejected rather than
given a precedence the notation does not define; write two constraints
instead. A feature may carry as many constraint lines as it needs.

Putting the constraint on its own line keeps one feature per entry line and
leaves indentation meaning hierarchy and nothing else — a constraint line never
takes part in the nesting, so the entry after it is a sibling of the entry
before it.

Requirements map to the JSON's `includes` field and exclusions to `excludes`.
Note that a requirement is **constructive** in featx, not merely declarative:
the configurator's `recursive_dependant_load` selects the targets rather than
only reporting their absence. A `||` requirement has no faithful JSON
counterpart, since the flat array is read as a conjunction; the generator
records `requiresOperator` so a consumer can tell.

## What it checks

Syntax, per document:

- a first line that is neither a root feature nor a configuration entry, or a
  root feature carrying a type or a binding
- an old `feature_model:` or `feature_config:` header, reported with the form
  that replaced it
- a missing `:`, a missing feature name, a missing or unknown type
- a node that declares no bindings, or a configuration entry that does not
- a colon or a type in a configuration entry, or anything after its binding
- unclosed `{`, `(` or `[`
- a missing or duplicated binding mode or binding time, or an unknown binding word
- a feature declared twice, or a marker left in its old position before the colon
- siblings indented to different columns
- a constraint with no operands, a leading, trailing or missing connective,
  mixed connectives, or a reserved word used as an operand
- a constraint with no entry above it to apply to, or indented away from it

Semantics, per document:

- `*` in a model — selection belongs in a configuration
- `?`, a grouping, or a constraint in a configuration — all belong in a model
- a grouping on a parameter, which can never have children
- a grouping on a node with no children, or with only one
- a value or a validator on a node, or children on a parameter
- a parameter with no enclosing bound node, so nothing is bound with it
- a value that does not suit its type: `double (fast)`, `int (1.5)`, `bool (yes)`
- a scalar given several values, or an array element of the wrong type
- a validator that cannot apply: a range on a non-numeric type, a size on a
  scalar, a validator on a node, an unknown name, the wrong number of
  arguments, a validator written twice, a comparison or an enumeration beside
  `bounds<>`, a reversed range, a pair nothing can satisfy, or a `fixed_size`
  that disagrees with the value's length
- a default its own validator rejects: outside a range, or absent from an
  enumeration
- `||` in an exclusion, where the connective makes no difference
- a constraint naming a feature the model does not declare
- a feature that requires or excludes itself
- a feature that both requires and excludes the same target
- a requirement cycle, reported with the full path
- a target listed twice in one clause
- a redundant requirement: an enclosing feature, or one that encloses another
  target in the same clause, since requiring a feature implies its ancestors
- an exclusion between a feature and its own ancestor or descendant, which
  makes the inner one unselectable

Against the feature model, for a configuration:

- a feature the model does not declare
- a node configured without a binding, or a parameter configured with one
- a configured value that does not suit the type the model declared
- a binding mode or time the model does not allow
- a selected feature whose parent is not selected
- a selected node whose typed parameters are not selected, or are omitted
- a selected feature whose requirements are not selected, or are omitted
  entirely; for a `||` requirement, that none of its operands is selected
- two features that exclude each other both selected
- two or more selected children inside an `xor` group (an `or` group places
  no upper bound, so nothing is reported for one)
- a hint listing model features the configuration omits

The model is found by scanning the configuration's own directory for a `.ftx`
file whose first line declares a root feature, so neither file has to be named in any
particular way. An open editor buffer wins over what is on disk, so unsaved
edits to a model are reflected in the configuration's diagnostics immediately.

## Deliberate choices

These are places where the specification left room, resolved so that the
project's own `features.ftx` and `configs.ftx` analyse clean:

- **`xor` means *at most* one**, not exactly one. `configs.ftx` selects
  `navigation` while selecting none of its three alternatives; treating `xor`
  as "exactly one" would flag that, so only a second selection is an error.
- **No mandatory-child rule for nodes.** A child without `?` is mandatory in
  feature-model theory, which would require every node child of a selected
  parent to be selected. `configs.ftx` does not satisfy that, so the rule is
  not enforced. Parameters are the exception: their selection *is* forced, by
  the rule above.
- **`or` has no lower bound.** It is read as "no upper bound on how many", not
  the conventional "one or more", so a selected `or` node with no selected
  children is not reported.
- **Binding order is not fixed.** `static early` and `early static` both parse.
  Every file in the repository writes mode first; nothing rejects the reverse.
- **No comment syntax.** The specification defines none, so no character starts
  a comment. Adding one means handling it in `tokenize`.

## Editor setup

The server has no opinion about file types; point your client at `*.ftx`.

**VS Code** needs an extension, since it has no setting for pointing at an
arbitrary LSP binary. One is included: see
[`editors/vscode`](editors/vscode/README.md). It adds syntax highlighting as
well, which the server does not provide.

**Neovim** (0.11+), in `init.lua`:

```lua
vim.filetype.add({ extension = { ftx = "featx" } })

vim.lsp.config.featx = {
  cmd = { "featx-lsp" },
  filetypes = { "featx" },
  root_markers = { ".git" },
}

vim.lsp.enable("featx")
```

**Helix**, in `languages.toml`:

```toml
[[language]]
name = "featx"
scope = "source.featx"
file-types = ["ftx"]
roots = [".git"]
language-servers = ["featx-lsp"]

[language-server.featx-lsp]
command = "featx-lsp"
```

## Layout

| Path | Contents |
|---|---|
| `cmd/featx-lsp` | the binary: flags, stdio wiring, `-lint`, `-emit`, `-params`, `-sync`, exit status |
| `internal/ftx` | the DSL — tokeniser, parser, analyser, formatter |
| `internal/lint` | the full check pipeline, including cross-file model lookup |
| `internal/emit` | rendering documents as featx JSON and as parameter schemas |
| `internal/ament` | reading what a built workspace provides, for `-implementations` |
| `internal/jsonrpc` | `Content-Length` framing and JSON-RPC 2.0 messages |
| `internal/lsp` | the subset of LSP types the server uses |
| `internal/server` | request dispatch and the language features |
| `editors/vscode` | the VS Code extension (a thin client plus highlighting) |

`internal/ftx` knows nothing about LSP beyond using its position convention
(0-based lines, UTF-16 character offsets), so it is usable on its own — as a
`.ftx` linter in CI, for instance.

## Tests

```
go test ./...
```

The suite covers the tokeniser and parser (including positions in UTF-16 code
units), formatter idempotency and stability against the real files, the
JSON-RPC framing, the generators and the installer, and a scripted end-to-end
LSP session per
language feature driven over an in-memory pipe. Diagnostics are asserted by
their exact message — around seventy of them — so a reworded message fails the
build rather than silently drifting from this document.

Two assertions are load-bearing: `internal/ftx` checks that the project's real
`features.ftx` and `configs.ftx` analyse without errors, and that formatting
either is a no-op. A rule or a format change that would reject them fails the
build.
