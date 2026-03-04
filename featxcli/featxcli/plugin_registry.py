import sys
import rclpy
import importlib
import traceback
from rclpy.node import Node
from featx_interfaces.srv import LoadFeature, UnloadFeature

class PluginRegistry(Node):
    def __init__(self):
        super().__init__('rclpy_plugin_registry')
        self.loaded_plugins = {}

        # ROS services
        self.load_srv = self.create_service(LoadFeature, 'load_feature', self.load_feature_callback)
        self.unload_srv = self.create_service(UnloadFeature, 'unload_feature', self.unload_feature_callback)
        self.get_logger().info('Python plugin registry activated')

    def load_feature_callback(self, request, response):
        module_name = request.module_name
        class_name = request.class_name

        try:
            if module_name in self.loaded_plugins:
                msg = f"Feature '{module_name}' already loaded."
                self.get_logger().info(msg)
                response.success = True
                response.message = msg
                self.get_logger().info(msg)
                return response

            if module_name in sys.modules:
                plugin_module = sys.modules[module_name]
                self.get_logger().info(f"Using already loaded module: {module_name}")
            else:
                self.get_logger().info(f"Importing new module: {module_name}")
                plugin_module = importlib.import_module(module_name)

            plugin_class = getattr(plugin_module, class_name)
            plugin_instance = plugin_class()

            self.loaded_plugins[module_name] = {
                "module": plugin_module,
                "class": plugin_class,
                "instance": plugin_instance,
            }

            self.get_logger().info(f"Loaded {module_name}.{class_name} successfully")
            response.success = True
            response.message = f"Loaded {module_name}.{class_name} successfully"

        except Exception as e:
            tb = traceback.format_exc()
            self.get_logger().error(f"Failed to load {module_name}: {e}\n{tb}")
            response.success = False
            response.message = str(e)

        return response
    

    def unload_feature_callback(self, request, response):
        module_name = request.module_name

        if module_name not in self.loaded_plugins:
            msg = f"Feature '{module_name}' is not loaded."
            self.get_logger().warning(msg)
            response.success = False
            response.message = msg
            return response

        plugin_data = self.loaded_plugins.pop(module_name)
        instance = plugin_data["instance"]

        #destroy the ROS node if it's a Node subclass
        if isinstance(instance, Node):
            self.get_logger().info(f"Destroying node for {module_name}")
            instance.destroy_node()

        #remove module from sys.modules to allow reloads
        if module_name in sys.modules:
            del sys.modules[module_name]
            self.get_logger().info(f"Removed {module_name} from sys.modules")

        response.success = True
        response.message = f"Unloaded {module_name}"
        self.get_logger().info(f"Unloaded feature {module_name} successfully")
        return response

def main(args=None):
    rclpy.init(args=args)
    node = PluginRegistry()
    rclpy.spin(node)
    rclpy.shutdown()

if __name__ == '__main__':
    main()