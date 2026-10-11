# FeatX
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This work presents a novel technique to load and unload robot features to and from a configuration. It extends the concept of feature models to support flexible feature binding. This is achieved by combining time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
`featxcli`: provides infrastructure for the featx command and its verbs integrated into ros2cli

`featxbinder`: rclcpp module responsible for managing manual and plugin compositions of features, and home of the generated `static_features.h` that decides which features are compiled in

`rclpy_components`: supplies for rclpy what `rclcpp_components` supplies for rclcpp — a container that loads and unloads Python nodes on request

`featx-lsp`: a language server for the `.ftx` feature-modelling DSL, which also generates the JSON the toolchain reads and `generate_parameter_library` schemas

To **bind** a feature is to commit its selection into the running system:
resolve which package implements it, make it present and running, and record
that it is now bound. When and how irrevocably that happens is what a feature's
binding clause decides. [How binding works](#how-binding-works) covers this in
full; the guide below gets you to the point of doing it.

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

## Implementing a feature

Declaring a feature in the model says it exists. Implementing it is a separate
step, and **how** you implement it is decided by the bindings you gave it: the
binding clause chooses the mechanism, and the wrong mechanism means the feature
cannot be bound at all.

### Only a `node` is implemented in code

A feature's type says what kind of thing it is, and only one kind has an
implementation:

```
xor navigation : node {static early} ?       a node: you write code for this
    vel_max : double (1.6) bounds<0.0, 3.0>  a parameter: you do not
    to_loading : node {dynamic late}         a node: you write code for this
```

A feature typed `node` is a ROS node. It has bindings, it is something you
write, and it is what `featx load` binds.

Every other type — `bool`, `int`, `double`, `string` and the four array types —
declares a **ROS parameter** of the node that encloses it. There is no class to
write and nothing to register. A parameter has no bindings of its own either,
because it is bound with its node: it comes up when the node comes up, and the
node's binding mode decides whether it can be changed afterwards. What you do
with a parameter is read it, which **Giving a feature parameters** below
covers.

So when reading the rest of this section, "feature" means a feature typed
`node`. Nothing here applies to a parameter.

### Where implementations live

Under `src/packages/`, one ROS package per feature bound **static early**, named
after that feature. The features beneath it in the model are components inside
that same package:

```
src/packages/
└── elevator/                       the static early feature `elevator`
    ├── include/elevator/
    │   └── elevator.hpp            the Elevator node
    ├── src/
    │   ├── elevator.cpp            the manual composition (an executable)
    │   ├── lift.cpp                the `lift` feature (a component)
    │   └── drop.cpp                the `drop` feature (a component)
    ├── CMakeLists.txt
    └── package.xml
```

So `elevator`, `lift` and `drop` are three features in one package. Nothing
makes you arrange it this way, but two things follow from it: a feature bound
early needs a package of its own to be launched, and `featx-lsp -sync` looks
for a parameter's package by the feature's name and then by the names of the
features it sits under.

### The name is the contract

Nothing records which package implements which feature. It is discovered, by
matching the feature's name against the registered classes: a feature's name in
`PascalCase` must be the class name.

| feature in the model | class to write |
|---|---|
| `lift` | `Lift` |
| `to_loading` | `ToLoading` |
| `init_checks` | `InitChecks` |

Underscores and hyphens become word breaks. The namespace is ignored, so
`featx_plugin::` is a convention rather than a requirement. Get the name wrong
and `featx load` reports the feature as having no implementation, which is the
most common mistake when adding one.

### A feature bound `static early` — a manual composition

This feature is compiled into an executable and launched with the system, so it
is written as a node that the executable instantiates itself. It is **not** a
component, and cannot be loaded or unloaded.

The node, in `include/elevator/elevator.hpp`:

```cpp
#include "rclcpp/rclcpp.hpp"

class Elevator: public rclcpp::Node{
    public:
        Elevator():Node("elevator"){
            RCLCPP_INFO(this->get_logger(), "Hello elevator");
        }
};
```

The manual composition, in `src/elevator.cpp`. The `#ifdef` is the mechanism
that keeps a deselected feature out of the binary, and the macro is the
feature's name upper-cased:

```cpp
#include "rclcpp/rclcpp.hpp"
#include "featxbinder/static_features.h"

#ifdef ELEVATOR
    #include "../include/elevator/elevator.hpp"
#elif !defined(ELEVATOR)
    #pragma message("Static feature Elevator not selected")
#endif

int main(int argc, char **argv){
    rclcpp::init(argc, argv);
    rclcpp::executors::SingleThreadedExecutor executor;

    #if ELEVATOR
        auto elevator = std::make_shared<Elevator>();
        executor.add_node(elevator);
    #endif

    executor.spin();
    rclcpp::shutdown();
    return 0;
}
```

`static_features.h` is generated by `ros2 featx start_config` from the
configuration — never edit it. If the feature is selected its macro is defined
and the node is compiled in; if not, the executable still builds and runs but
instantiates nothing, and the compiler says so. Either way the feature cannot
be switched on without a rebuild, which is what `static early` means.

In `CMakeLists.txt`, the executable is named after the feature:

```cmake
find_package(featxbinder REQUIRED)       # for static_features.h

add_executable(elevator src/elevator.cpp)
ament_target_dependencies(elevator rclcpp featxbinder)

install(TARGETS elevator RUNTIME DESTINATION lib/${PROJECT_NAME})
```

and in `package.xml`, `<depend>featxbinder</depend>`.

### Every other binding — a component

`static late`, `dynamic early` and `dynamic late` features are all loaded into
a container at runtime by `featx load`. They are components, in either
language, and they register themselves so that no list has to be maintained.

**In C++**, a node taking `NodeOptions`, in `src/lift.cpp`:

```cpp
#include "rclcpp/rclcpp.hpp"

namespace featx_plugin{
    class Lift: public rclcpp::Node{
        public:
            Lift(const rclcpp::NodeOptions &options):Node("lift", options){
                RCLCPP_INFO(this->get_logger(), "Hello from lift feature");
            }
    };
}

#include "rclcpp_components/register_node_macro.hpp"
RCLCPP_COMPONENTS_REGISTER_NODE(featx_plugin::Lift)
```

The `NodeOptions` parameter is what makes it loadable; a node without it cannot
be composed. In `CMakeLists.txt`:

```cmake
add_library(lift SHARED src/lift.cpp)
ament_target_dependencies(lift rclcpp rclcpp_components)
rclcpp_components_register_nodes(lift "featx_plugin::Lift")

install(TARGETS lift LIBRARY DESTINATION lib)
```

`rclcpp_components_register_nodes` is the line that matters: it writes the
ament index entry that `featx load` finds. Without it the library is built and
installed and the feature is still undiscoverable.

**In Python**, the equivalent of `NodeOptions` is `**kwargs` forwarded to
`Node`, in `py_plugin_executor/boot.py`:

```python
import rclpy
from rclpy.node import Node


class Boot(Node):
    def __init__(self, **kwargs):
        kwargs.setdefault("node_name", "boot")
        super().__init__(**kwargs)
        self.get_logger().info("boot feature activated")
```

The `**kwargs` is load-bearing. The container passes the node name, namespace,
parameters and remappings from the load request through it; a class that takes
no arguments is constructed bare, and any options sent for it are refused
rather than silently dropped.

Python has no CMake to generate the index entry, so the package carries a file
listing what it provides — `components/py_plugin_executor`, one line per
feature as `<name>;<module>:<Class>`:

```
featx_plugin::Boot;py_plugin_executor.boot:Boot
featx_plugin::InitChecks;py_plugin_executor.init_checks:InitChecks
```

and installs it in `setup.py`:

```python
data_files=[
    ('share/ament_index/resource_index/rclpy_components',
        ['components/' + package_name]),
]
```

That path is what makes it discoverable; `rclpy_components` is the group
`featx load` reads for Python features, beside `rclcpp_components` for C++.

**Any package may provide Python features.** `py_plugin_executor` is not
special — nothing in the tooling names it, and the group is read across every
package that registers in it, so a feature belongs in the package that suits
the model rather than being collected in one place. Its name is a leftover from
an earlier design in which a single process hosted the Python features itself.

Note that a Python feature cannot be bound `static early`. That binding is
compiled in, and there is nothing to compile; see **Feature binding in both
languages** below. Any other combination is open to either language, and
`featx-lsp -lint -implementations` reports the model if a `static early`
feature turns out to be implemented in rclpy.

### Giving a feature parameters

A typed feature in the model is a ROS parameter of the node that receives it.
Declare it in `features.ftx` with a default and, if you want one, a validator:

```
lift : node {dynamic late}
    max_height : double (2.0) bounds<0.0, 4.0>
```

Deliver the schema to the package, then wire it up:

```bash
cd src/featx_suite/featx-lsp
./featx-lsp -sync -w -into ../../packages ../featxcli/featxcli/model/features.ftx
```

`-sync` prints the two or three lines to add to that package's `CMakeLists.txt`
and `package.xml`, and keeps printing them until they are there. Then read the
parameters in the node:

```cpp
#include "lift_parameters.hpp"
// in the constructor:
param_listener_ = std::make_shared<lift::ParamListener>(
    this->get_node_parameters_interface());
params_ = param_listener_->get_params();   // params_.max_height
```

The generated code declares the parameter, applies the default and enforces the
validator, so setting `max_height` to `9.9` on the running node is rejected
because the model said `bounds<0.0, 4.0>`.

### Checklist for a new feature

1. Declare it in `features.ftx` with a type and, for a node, its bindings. A
   parameter stops here and at step 3: steps 4 to 6 are for nodes only.
2. Add an entry to `configs.ftx`, with `*` if it should start out selected.
3. `featx-lsp -lint`, then `-emit -w` both model files.
4. Implement it by the mechanism its bindings call for: a manual composition
   for `static early`, a component otherwise.
5. Name the class after the feature in `PascalCase`.
6. Register it — `rclcpp_components_register_nodes` in CMake, or a line in the
   `components/` file for Python.
7. `colcon build && source install/setup.bash`.
8. Confirm nothing is missing:

   ```bash
   featx-lsp -lint -implementations featxcli/featxcli/model
   ```

   This checks each node against the built workspace and names any that has no
   implementation, and the class name it looked for. It is how a misnamed class
   is caught before `featx load` fails on it.
9. `ros2 featx start_config` for a `static early` feature, otherwise
   `ros2 featx load -f <feature>` on the running system.

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

**`featx load` says one particular feature has no implementation.** Usually
the class is not named after the feature: a feature's name in `PascalCase` is
the class that must exist, so `to_loading` needs `ToLoading`. Run `featx-lsp
-lint -implementations` on the model to be told which features are missing and
what each one was looked up as. The other possibility is that the component was
never registered, so the code built and installed but announced nothing.

**`Fix issue(s), build and rerun the command again`.** The configuration
contradicts itself — a selected child under a deselected parent, or a binding
that disagrees with the model. The lines above the message name the features
involved.

## How binding works

### What it means to bind a feature

Binding is the act of **committing a feature's selection into the running
system** — turning "this feature is selected" into "this feature is present and
doing something". The feature model says what could be; binding is what makes
it so.

Binding one feature does three things:

1. **Resolution** — find the implementation. The feature's name is looked up in
   the ament index, which says which package provides it and in which language.
   Nothing declares that mapping; it is derived from the name.
2. **Activation** — make it present and running. Either compiled into an
   executable that is launched, or loaded into a container and added to its
   executor so its callbacks actually run.
3. **Recording** — update the configuration, so the model stays the truth about
   what is bound.

That third step is why `ros2 featx load` is worth preferring over
`ros2 component load`: the latter activates a component without recording
anything, leaving the model disagreeing with the system it describes.

The binding clause then answers two questions, and they are independent:

| axis | question | values |
|---|---|---|
| **time** | *when* does the commitment happen? | `early` — at build and launch · `late` — while the system runs |
| **mode** | *can it be revoked* afterwards? | `static` — fixed once bound · `dynamic` — can be rebound |

So `static` does not mean "starts early", and `early` does not mean
"permanent". Keeping them apart is the point: it is what lets one system carry
features committed at four different strengths, from compiled in and
unchangeable to loaded and unloaded at will. The next section says what each
combination does.

Two things follow that are easy to miss.

**The system knows which phase it is in.** `featx_binder` carries a
`bindingTime` parameter, which becomes `Late` once the configuration is up, and
`load` and `unload` check it before doing anything. Early and late are
therefore not only labels on a model — there is a runtime notion of which
binding phase the system is in, and a late binding is refused outside it.

**A parameter is never bound on its own.** A typed feature has no binding
clause because it is bound with the node that encloses it: it appears when that
node appears, and the node's *mode* decides whether it can be changed
afterwards. That is why `read_only` in a generated parameter schema is derived
from the binding mode of the node that receives the parameter, rather than
written by hand.

Put plainly: ROS 2 gives you compile-time composition or runtime composition,
and which one you get is decided by how the code was written. FeatX makes
binding time and mode properties of the *feature model* instead, and the
toolchain enforces what each combination admits — including refusing to unload
a feature declared static, and refusing to load one that was compiled in.

### What each combination does
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
    vel_max : double (1.6) bounds<0.0, 3.0>
    to_loading : node {dynamic late}
```

`featx-lsp -sync` renders that as a
[generate_parameter_library](https://github.com/PickNikRobotics/generate_parameter_library)
specification and delivers it to the package that builds the node, which
generates C++ enforcing both the default and the bounds. Setting `vel_max` to
`9.9` on a running node is then rejected, because the model said
`bounds<0.0, 3.0>`.

Each validator is written by the name `generate_parameter_library` gives it, so
the model and the schema use the same word: `bounds<0.0, 3.0>` admits both
ends, `gt<0.0> lt<3.0>` neither, and `gt<0.0> lt_eq<3.0>` only the upper one. A
parameter may carry several, which the library applies in turn, and a `string`
or an array may constrain its length with `fixed_size<8>`, `size_gt<2>`,
`size_lt<20>` or `not_empty<>`.

`bounds<0.0, 3.0>` stands alone: it gives both ends, so a comparison or an
enumeration beside it would state a limit the parameter already has. Write the
pair instead when one end is open.

Whether a parameter can be changed once set follows its node's binding mode,
and the schema says so either way: `read_only: true` under a `static` node,
which cannot be rebound, and `read_only: false` under a `dynamic` one.

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

The two languages are equal for everything bound at runtime: the same verbs,
the same containers, the same contract. They are not equal everywhere, and two
differences are irreducible rather than unfinished.

**A Python feature cannot be bound `static early`.** That binding is
implemented by the preprocessor: `static_features.h` defines a macro per
selected feature, and the manual composition is compiled behind `#ifdef`, which
is what makes a deselected feature absent from the binary. Python has no
compile step, so there is nothing to leave out. `boot` and `init_checks` are
`static late` for this reason, not by preference.

You can get *early* in Python — `early.launch.py` launches
`Node(package=feature, executable=feature)`, which works for an `ament_python`
package with a console script of that name — but not the *static* part. A
deselected feature would simply not be launched while remaining installed and
startable by hand, which is the opposite of the guarantee `static early`
exists to give. So compile-time binding is C++ only, and runtime binding is
available to both.

`featx-lsp -lint -implementations` enforces this, so the mistake is caught at
the model rather than becoming a deployment that quietly carries a feature it
was configured without.

**A C++ component genuinely unloads, and a Python one does not quite.** Its
library can be closed; Python has no `dlclose`, so unbinding an rclpy feature
stops its callbacks and destroys its node, but the imported module stays
resident. The feature is unbound in every way that is observable over ROS, and
the memory is not reclaimed.

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
