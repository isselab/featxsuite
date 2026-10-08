import importlib
import traceback

import rclpy
from rclpy.node import Node
from rclpy.executors import MultiThreadedExecutor
from composition_interfaces.srv import LoadNode, UnloadNode, ListNodes


class PyComponentContainer(Node):
    """A Python container speaking the same service contract as rclcpp_components.

    Serving composition_interfaces means `ros2 component load/unload/list`
    drives this container directly, so featx needs no separate code path for
    rclpy features.

    plugin_name is `module:Class` relative to package_name, e.g.
    `ros2 component load /featx_py_container py_plugin_executor boot:Boot`
    """

    def __init__(self):
        super().__init__('featx_py_container')
        self.components = {}
        self.next_unique_id = 1

        # these three share the node's default mutually-exclusive callback group, which is
        # what serializes access to components/next_unique_id under MultiThreadedExecutor
        self.create_service(LoadNode, '~/_container/load_node', self.load_node_callback)
        self.create_service(UnloadNode, '~/_container/unload_node', self.unload_node_callback)
        self.create_service(ListNodes, '~/_container/list_nodes', self.list_nodes_callback)

        self.get_logger().info('featx python component container activated')

    def load_node_callback(self, request, response):
        unsupported = self.unsupported_options(request)
        if unsupported:
            response.success = False
            response.error_message = (
                f"this container cannot apply {', '.join(unsupported)}: python plugins are "
                f"constructed with no arguments"
            )
            self.get_logger().error(response.error_message)
            return response

        try:
            module_name, _, class_name = request.plugin_name.partition(':')
            if not class_name:
                raise ValueError(f"plugin_name '{request.plugin_name}' is not 'module:Class'")

            qualified_module = f"{request.package_name}.{module_name}"
            plugin_module = importlib.import_module(qualified_module)
            instance = getattr(plugin_module, class_name)()

            if not isinstance(instance, Node):
                instance_type = type(instance).__name__
                raise TypeError(f"{request.plugin_name} is a {instance_type}, not an rclpy Node")

            self.executor.add_node(instance)

            unique_id = self.next_unique_id
            self.next_unique_id += 1
            self.components[unique_id] = instance

            response.success = True
            response.full_node_name = instance.get_fully_qualified_name()
            response.unique_id = unique_id
            self.get_logger().info(f"loaded {response.full_node_name} as {unique_id}")

        except Exception as e:
            self.get_logger().error(f"failed to load {request.plugin_name}: {traceback.format_exc()}")
            response.success = False
            response.error_message = str(e)

        return response

    def unload_node_callback(self, request, response):
        instance = self.components.pop(request.unique_id, None)

        if instance is None:
            response.success = False
            response.error_message = f"no loaded component with unique_id {request.unique_id}"
            self.get_logger().warning(response.error_message)
            return response

        self.executor.remove_node(instance)
        instance.destroy_node()

        response.success = True
        self.get_logger().info(f"unloaded component {request.unique_id}")
        return response

    def list_nodes_callback(self, request, response):
        response.unique_ids = list(self.components.keys())
        response.full_node_names = [
            instance.get_fully_qualified_name() for instance in self.components.values()
        ]
        return response

    def unsupported_options(self, request):
        """NodeOptions fields a no-argument plugin constructor cannot receive."""
        named = []
        if request.node_name:
            named.append('node_name')
        if request.node_namespace:
            named.append('node_namespace')
        if request.remap_rules:
            named.append('remap_rules')
        if request.parameters:
            named.append('parameters')
        if request.extra_arguments:
            named.append('extra_arguments')
        return named


def main(args=None):
    rclpy.init(args=args)
    node = PyComponentContainer()
    executor = MultiThreadedExecutor()
    executor.add_node(node)
    try:
        executor.spin()
    finally:
        executor.shutdown()
        rclpy.shutdown()


if __name__ == '__main__':
    main()
