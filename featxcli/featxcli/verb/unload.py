from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess, re
from rclpy.logging import get_logger
from featxcli.containers import CPP_CONTAINER, find_component_id

configurator = Configurator()
logger = get_logger('unload')
ILIB_PLUGINS_EXECUTOR_PKG = 'py_plugin_executor'


class UnloadVerb(VerbExtension):
    """The verb for deselecting a configured feature."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument('-f', '--feature', type=str, required=True, help='Unloading a configured feature from a configuration')

    def run_ros2_featx_unload_command(self, feature_name):
        conflicts = configurator.checkRules()

        if conflicts == 0:

            result = subprocess.run(["ros2", "node", "list"], capture_output=True, text=True)
            nodes = result.stdout.strip().split('\n')

            if len(nodes) == 1 and nodes[0] == "":
                logger.info("Configuration is not running. Run ros2 featx start_config")
            else:
                try:
                    bt_command = ["ros2", "param", "get", "/featx_binder", "bindingTime"]
                    bt_result = subprocess.run(bt_command, capture_output=True, text=True, check=True)
                    bt_value = bt_result.stdout.split(":")[1].strip()

                    if bt_value == "Late":
                        #run unload command
                        fmode = configurator.get_binding_mode(feature_name)

                        if fmode == "Static":
                            logger.info("Warning: Cannot unload a static feature at runtime!")
                            
                        else:
                            pass
                            #get includes
                            include_list = configurator.find_feature_includes(feature_name)
                            #recursively unload feature and it includes
                            
                            if isinstance(include_list, str):
                                if len(include_list) > 0:
                                    logger.info(f"Unloading dependant: {include_list}")
                                    self.shut_node_down(include_list)

                            elif isinstance(include_list, list):
                                for dependant in include_list:
                                    logger.info(f"Unloading dependant: {dependant}")
                                    self.shut_node_down(dependant) 

                            #finally shut down main node
                            self.shut_node_down(feature_name)
                    else:
                        logger.info("Binding time not late. Cannot run this command at compile time")

                except subprocess.CalledProcessError as e:
                    logger.info("Could not get live bindingTime value:")
                    logger.info(e.stderr)

        else:
            logger.info("Fix issue(s), build and rerun the command again")

    def call_unload_service(self, module_name):
        object_data = f"{{module_name: '{module_name}'}}"
        completed = subprocess.run(['ros2', 'service', 'call', '/unload_feature', 'featx_interfaces/srv/UnloadFeature', object_data],capture_output=True, text=True)

        #ros2 service call exits 0 even when the service itself reports failure
        if completed.returncode != 0:
            return False, completed.stderr.strip()

        success = re.search(r"success=(True|False)", completed.stdout)
        message = re.search(r"message='([^']*)'", completed.stdout)

        return (success is not None and success.group(1) == "True",
                message.group(1) if message else "no message in response")

    def shut_node_down(self, feature_name):
        logger.info(f"*** Unloading feature: {feature_name}...***")

        comp_id = find_component_id(CPP_CONTAINER, feature_name)

        if comp_id is not None:
            unload_result = subprocess.run(["ros2", "component", "unload", CPP_CONTAINER, comp_id], capture_output=True, text=True)

            if unload_result.returncode == 0:
                logger.info(unload_result.stdout.strip())
                configurator.updateConfigModelSelection(feature_name, False)
                logger.info(f"{feature_name} unloaded successfully.")
            else:
                logger.info(unload_result.stderr.strip())

            return

        #not an rclcpp component, so ask the rclpy plugin registry
        package_name = configurator.get_parent(feature_name)

        if package_name == "root":
            package_name = ILIB_PLUGINS_EXECUTOR_PKG

        #a feature's module sits either in py_plugin_executor or in a package of its own name
        for module_name in (f"{package_name}.{feature_name}", f"{feature_name}.{feature_name}"):
            logger.info(f"Check if module exists at: {module_name}")
            success, message = self.call_unload_service(module_name)
            logger.info(f"Service message: {message}")

            if success:
                configurator.updateConfigModelSelection(feature_name, False)
                logger.info(f"{feature_name} unloaded successfully.")
                return

        logger.info(f"{feature_name} could not be unloaded as a component or an rclpy plugin")


    def check_if_feature_exists(self, feature_name):
        return configurator.find_feature(feature_name)
    
    def check_if_config_exists(self, name_target):
        cparsed = configurator.readConfigs()
        match = next((cfg["name"] for cfg in cparsed["configs"] if cfg["name"] == name_target), None)
        return match
    
    def main(self, *, args):
        feature_exists = self.check_if_feature_exists(args.feature)

        if feature_exists:
            logger.info(f"{feature_exists} feature exists")
            config_exists = self.check_if_config_exists(args.feature)

            if config_exists:
                logger.info(f"{feature_exists} configuration exists\n")
                self.run_ros2_featx_unload_command(args.feature)
            else:
                logger.info(f"Configuration for {feature_exists} doesnot exist. If it is not an abstract feature, add a configuration in model/configs.json")
        else:
            logger.info(f"\t--- Feature({str(args.feature)}) has not been defined in features.json ---")

