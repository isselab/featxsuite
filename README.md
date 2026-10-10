# FeatX
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This work presents a novel technique to load and unload robot features to and from a configuration. It extends the concept of feature models to support flexible feature binding. This is achieved by combining time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
`featxcli`: provides infrastructure for the featx command and its verbs integrated into ros2cli

`featxbinder`: rclcpp module responsible for managing manual and plugin compositions of features, and home of the generated `static_features.h` that decides which features are compiled in

`rclpy_components`: supplies for rclpy what `rclcpp_components` supplies for rclcpp — a container that loads and unloads Python nodes on request

`featx-lsp`: a language server for the `.ftx` feature-modelling DSL, which also generates the JSON the toolchain reads and `generate_parameter_library` schemas

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
