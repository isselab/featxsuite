# FeatX
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This work presents a novel technique to load and unload robot features to and from a configuration. It extends the concept of feature models to support flexible feature binding. This is achieved by combining time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
`featxcli`: provides infrastructure for the featx command and its verbs integrated into ros2cli

`featxbinder`: rclcpp module responsible for managing manual and plugin compositions of features

`featx_interfaces`: custom rclpy infrastructure that makes provision for rclpy plugins via services 

### Python feature binding
ROS 2 provides no rclpy counterpart to `rclcpp_components`: a component container binds C++ composable nodes through `class_loader`, which has no Python equivalent. FeatX closes that gap with `featx_interfaces`, whose `LoadFeature` and `UnloadFeature` services are served by the `plugin_registry` node. Binding a Python feature imports its module, instantiates the class and adds the node to the registry's executor so that its callbacks run; unbinding removes it from the executor and destroys the node.

`featxcli` also ships `component_container`, an alternative registry that serves the standard `composition_interfaces` contract (`LoadNode`, `UnloadNode`, `ListNodes`) instead of the FeatX services. Run it in place of `plugin_registry` and rclpy features become visible to the same tooling as rclcpp ones:

```
ros2 run featxcli component_container
ros2 component load /featx_py_container py_plugin_executor boot:Boot
ros2 component unload /featx_py_container 1
```

Either way, prefer `ros2 featx load`/`unload` over the commands above: only the verbs validate the feature model before binding and record the resulting selection state. Note also that Python cannot truly unload code — unbinding stops a feature's callbacks and destroys its node, but the imported module remains resident.

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
