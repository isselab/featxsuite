# FeatX
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This work presents a novel technique to load and unload robot features to and from a configuration. It extends the concept of feature models to support flexible feature binding. This is achieved by combining time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
`featxcli`: provides infrastructure for the featx command and its verbs integrated into ros2cli

`featxbinder`: rclcpp module responsible for managing manual and plugin compositions of features

`rclpy_components`: supplies for rclpy what `rclcpp_components` supplies for rclcpp — a container that loads and unloads Python nodes on request

`featx-lsp`: a language server for the `.ftx` feature-modelling DSL, which also generates the JSON the toolchain reads and `generate_parameter_library` schemas

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
