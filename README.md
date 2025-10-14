# FeatX
This work presents a novel technique to load and unload robot features to and from a configuration. jIt combines time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
-`featxcli`: provides infrastructure for featx command and verbs integrated into ros2cli
-`featxbinder`: rclcpp module responsible for managing manual and plugin compositions
-`featx_interfaces`: makes provision for rclpy plugins 

## Documentation
