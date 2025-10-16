______         _        
|  ___|       | |       
| |_ ___  __ _| |___  __
|  _/ _ \/ _` | __\ \/ /
| ||  __/ (_| | |_ >  < 
\_| \___|\__,_|\__/_/\_\

This work presents a novel technique to load and unload robot features to and from a configuration. It extends the concept of feature models to support flexible feature binding. This is achieved by combining time and mode bindings to deliver a robust mechanism for managing variability in such systems. This technique is delivered via a domain-specific language that can be used to define features and bind them according to the user's prefereed time and mode bindings. The configuration and reconfiguration process is managed by a configurator tool, with the entire implementation integrated via the ros2cli. We evaluated our tool with case studies drawn from a simulated warehouse scenario.

## Overview
`featxcli`: provides infrastructure for the featx command and its verbs integrated into ros2cli

`featxbinder`: rclcpp module responsible for managing manual and plugin compositions of features

`featx_interfaces`: custom rclpy infrastructure that makes provision for rclpy plugins via services 

## Documentation
https://github.com/JGyimah/featxsuite/blob/main/documentation.pdf
