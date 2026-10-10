# FeatX
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This work presents a novel technique to load and unload robot features to and from a configuration. It extends the concept of feature models to support flexible feature binding. This is achieved by combining time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
`featxcli`: provides infrastructure for the featx command and its verbs integrated into ros2cli

`featxbinder`: rclcpp module responsible for managing manual and plugin compositions of features, and home of the generated `static_features.h` that decides which features are compiled in

`rclpy_components`: supplies for rclpy what `rclcpp_components` supplies for rclcpp — a container that loads and unloads Python nodes on request

`featx-lsp`: a language server for the `.ftx` feature-modelling DSL, which also generates the JSON the toolchain reads and `generate_parameter_library` schemas

## Prerequisites

| | |
|---|---|
| Ubuntu | 22.04 |
| ROS 2 | Iron Irwini |
| Python | 3.10 |
| Go | 1.23 or newer, and only for `featx-lsp` |

Everything else is a ROS package, listed under Dependencies below.

ROS 2 has to be on the shell before anything here will work, and it has to be
on *every* shell you use:

```bash
source /opt/ros/iron/setup.bash
```

Add that line to your `~/.bashrc` if you would rather not repeat it.

## Dependencies

`colcon` needs two extensions to see ROS packages at all. Without them a build
appears to succeed while quietly skipping most of the workspace:

```bash
sudo apt install python3-colcon-ros python3-colcon-cmake
```

The ROS packages the suite depends on:

```bash
sudo apt install \
  ros-iron-ros2cli \
  ros-iron-ros2component \
  ros-iron-composition-interfaces \
  ros-iron-generate-parameter-library
```

- **ros2cli** / **ros2component** — the `ros2` command, and the `ros2 component`
  verbs that `featx load` and `featx unload` drive.
- **composition_interfaces** — the `LoadNode`, `UnloadNode` and `ListNodes`
  services. Both containers serve these, which is how one code path binds
  features in either language.
- **generate_parameter_library** — turns a feature's typed parameters into C++
  that enforces the model's defaults and bounds.

`rclcpp`, `rclpy`, `rclcpp_components` and `ament_*` come with a desktop ROS 2
install. If you installed `ros-iron-ros-base` instead, add them.

Or let rosdep read the `package.xml` files and work it out:

```bash
cd ~/ros2_ws
rosdep install --from-paths src --ignore-src -y
```

## Workspace layout

Two repositories, side by side under `src/`:

```
~/ros2_ws/
└── src/
    ├── featx_suite/      this repository: the tool
    └── packages/         the feature implementations
```

`featx_suite` is the tool. `packages` holds the features themselves — one ROS
package per feature that is bound early, each also providing the components for
the features beneath it. The two are separate because the tool is not tied to
any particular robot: point it at a different `packages` and it configures that
instead.

The commands below assume `~/ros2_ws`. Any path works, as long as both
repositories sit under the same `src/`.

## Build

```bash
cd ~/ros2_ws
colcon build
source install/setup.bash
```

That last line is not optional, and it is the single most common thing to
forget. Features are discovered through the ament index, which lives in
`install/`, so an unsourced shell finds no features at all and `featx load`
reports every one of them as unregistered. Source it again in every new
terminal.

Check that the command registered:

```bash
ros2 featx --help
```

## A first run

### 1. Start the configuration

```bash
cd ~/ros2_ws
ros2 featx start_config
```

This does four things, in order:

1. Checks the configuration for contradictions, and stops if it finds any.
2. Writes `featxbinder`'s `static_features.h` with one macro per selected
   **static early** feature.
3. Builds those features, so the binary contains what the configuration asked
   for and nothing else.
4. Launches them, along with the two containers that host everything bound
   later.

It stays in the foreground. Leave it running and open a second terminal for
what follows — remembering to `source install/setup.bash` there too.

### 2. See what is running

```bash
ros2 node list
ros2 component list
```

`node list` shows the early features and the containers. `component list` shows
what has been loaded into `/featx_container` (C++) and `/featx_py_container`
(Python), which starts out empty.

### 3. Bind a feature

```bash
ros2 featx load -f boot
ros2 featx load -f to_loading
```

Each call validates the model, finds which package implements the feature and
in which language, loads it into the matching container, and records the
selection. Both commands look the same to you; `boot` is Python and
`to_loading` is C++.

`ros2 component list` will now show them.

### 4. Unbind one

```bash
ros2 featx unload -f to_loading
```

`to_loading` is bound **dynamically**, so this works. Try a static feature and
it will refuse:

```bash
ros2 featx unload -f boot
# Warning: Cannot unload a static feature at runtime!
```

That is the point of declaring it static: it is a promise that the feature will
not be rebound once it is up. See the binding table below for which features
can be unbound.

## Changing the configuration

Selection lives in `featxcli/featxcli/model/configs.ftx`, where `*` marks a
feature as selected:

```
navigation *{static early}
vel_max *
to_loading * {dynamic late}
to_delivery {dynamic late}
```

The toolchain reads JSON, which is generated from the DSL. After editing, emit
it:

```bash
cd ~/ros2_ws/src/featx_suite/featx-lsp
go run ./cmd/featx-lsp -lint ../featxcli/featxcli/model
go run ./cmd/featx-lsp -emit -w ../featxcli/featxcli/model/configs.ftx
```

Lint first: `-emit` refuses a model it would reject anyway, and the error
messages say what is wrong and where.

Whether you then need to rebuild depends on what you changed:

- **A static early feature** — rebuild. It is compiled in, so the selection
  only takes effect through `ros2 featx start_config`, which rebuilds for you.
- **Anything else** — no rebuild. Use `featx load` and `featx unload` on the
  running system.

## Editing the model

`features.ftx` and `configs.ftx` are the source of truth, and `featx-lsp`
understands them. Build it once:

```bash
cd ~/ros2_ws/src/featx_suite/featx-lsp
go build -o featx-lsp ./cmd/featx-lsp
```

Then check your edits from the terminal:

```bash
./featx-lsp -lint ../featxcli/featxcli/model
```

For live diagnostics, hover and completion while you type, install the VS Code
extension — `featx-lsp/editors/vscode/README.md` has the steps. The same checks
run either way, so a model that lints clean in the terminal is one the editor
accepts.

If you add a typed parameter, deliver its schema to the package that uses it:

```bash
./featx-lsp -sync -w -into ../../packages ../featxcli/featxcli/model/features.ftx
```

Run it without `-w` first to see what it would do.

## When something goes wrong

**`featx load` says no component is registered, for every feature.** The shell
has no workspace on it. `source install/setup.bash`.

**`Configuration is not running. Run ros2 featx start_config`.** Nothing is up
yet. Start it, and keep that terminal open.

**`colcon build` finishes suspiciously fast and builds only a few packages.**
The colcon extensions are missing. Install `python3-colcon-ros` and
`python3-colcon-cmake`, then build again.

**`start_config` cannot find the workspace.** It searches upward from where you
are for a `src/` containing `featxbinder`. Run it from inside the workspace, or
pass `--workspace ~/ros2_ws`.

**A feature you deselected is still there.** `colcon` installs but never
uninstalls, so its old binary survives in `install/`. `start_config` names any
such feature; remove `install/<feature>` and `build/<feature>` to be rid of it.

**`Fix issue(s), build and rerun the command again`.** The configuration
contradicts itself — a selected child under a deselected parent, or a binding
that disagrees with the model. The lines above the message name the features
involved.

## How binding works

### Binding time and mode decide how a feature is bound
A feature's binding clause is not a label; it selects the mechanism that binds
it, and what can be done to it afterwards.

| time | mode | how it is bound | unload |
|---|---|---|---|
| early | static | compiled into a manual composition and launched with the system | no — it is not a component |
| early | dynamic | loaded into a container as a component | yes |
| late | static | loaded into a container as a component | no — it is static |
| late | dynamic | loaded into a container as a component | yes |

**Static and early is a compile-time decision.** `ros2 featx start_config`
generates `featxbinder`'s `static_features.h` from the configuration, giving a
macro to each selected feature, and then builds those features and launches
them. A feature that is not selected gets no macro, so it is not merely
inactive — it is absent from the binary. Activating it takes a change to the
configuration, a rebuild and a redeploy, which is what keeps a deployment down
to the features it was configured for:

```
ros2 featx start_config                 # generate, build the selected features, launch
ros2 featx start_config --no-build      # launch without rebuilding
```

The header is generated, so editing it by hand has no lasting effect; change
the configuration instead. `start_config` finds the workspace by searching
upward from the current directory, and `--workspace` overrides that.

**Everything else is bound at runtime** with `ros2 featx load`, as a component
in whichever language implements it. `ros2 featx unload` reverses that, except
for a static feature: being static is precisely the claim that it will not be
rebound, so unloading one is refused.

```
ros2 featx load -f boot                 # static late, rclpy
ros2 featx load -f to_loading           # dynamic late, rclcpp
ros2 featx unload -f to_loading         # dynamic, so this is allowed
ros2 featx unload -f boot               # refused: boot is static
```

### Parameters come from the model
A typed feature is a ROS parameter, and it is declared once in the model rather
than restated in each package's own schema:

```
xor navigation : node {static early} ?
    vel_max : double (1.6) <0.0, 3.0>
    to_loading : node {dynamic late}
```

`featx-lsp -sync` renders that as a
[generate_parameter_library](https://github.com/PickNikRobotics/generate_parameter_library)
specification and delivers it to the package that builds the node, which
generates C++ enforcing both the default and the bounds. Setting `vel_max` to
`9.9` on a running node is then rejected, because the model said `<0.0, 3.0>`.

A parameter written at a grouping belongs to each of its alternatives, since
those are the nodes that run and a ROS parameter belongs to one node.

### Feature binding in both languages
ROS 2 provides no rclpy counterpart to `rclcpp_components`: a container binds C++ composable nodes through `class_loader`, which has no Python equivalent. FeatX closes that gap with `rclpy_components`, a container that serves the same `composition_interfaces` contract (`LoadNode`, `UnloadNode`, `ListNodes`) over the same service names, so the standard tooling drives it:

```
ros2 component load /featx_py_container py_plugin_executor featx_plugin::Boot
ros2 component list
ros2 component unload /featx_py_container 1
```

Components advertise themselves through the ament resource index — `rclcpp_components` for C++, `rclpy_components` for Python — so a feature's implementing package and language are discovered rather than declared. `ros2 featx load`/`unload` consult that index and load into the matching container, which means both languages take one code path.

Prefer the verbs over the commands above: only they validate the feature model before binding and record the resulting selection state.

One difference between the two languages is irreducible. A C++ component genuinely unloads, because its library can be closed. Python has no `dlclose`, so unbinding an rclpy feature stops its callbacks and destroys its node, but the imported module remains resident.

## Project Website
https://jgyimah.github.io/featx/

```
 ███████████                     █████    █████ █████
░░███░░░░░░█                    ░░███    ░░███ ░░███ 
 ░███   █ ░   ██████   ██████   ███████   ░░███ ███  
 ░███████    ███░░███ ░░░░░███ ░░░███░     ░░█████   
 ░███░░░█   ░███████   ███████   ░███       ███░███  
 ░███  ░    ░███░░░   ███░░███   ░███ ███  ███ ░░███ 
 █████      ░░██████ ░░████████  ░░█████  █████ █████
░░░░░        ░░░░░░   ░░░░░░░░    ░░░░░  ░░░░░ ░░░░░ 
```
