from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess, os, re
from ament_index_python.packages import get_package_share_directory
from rclpy.logging import get_logger
from featxcli.executor_list import EXECUTOR_LIST


configurator = Configurator()
logger = get_logger('load')
project_name = 'packages'
#not to be changed
ILIB_PLUGINS_DIRECTORY = 'py_plugin_executor'
ILIB_PLUGINS_EXECUTOR_PKG = 'py_plugin_executor'

class LoadVerb(VerbExtension):
    """The verb for selecting a configured feature."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument('-f', '--feature', type=str, required=True, help='Loading a configured feature into a configuration')
        
    def run_ros2_featx_load_command(self, feature_name):
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
                        
                        fmode = configurator.get_binding_mode(feature_name)
                        ftime = configurator.get_binding_time(feature_name)

                        if fmode == "Static" and ftime == "Early":
                        
                            component_found = self.check_running_component_list(feature_name)
                            node_found = self. check_running_node_list(feature_name)

                            if component_found == "no" and node_found == "no":
                                logger.info(f"Warning: Cannot load a static early feature({feature_name}) at runtime!")  
                            
                        else:
                            logger.info(f"*** Loading feature: {feature_name}...***")

                            if configurator.get_parent(feature_name) == 'root':
                                if feature_name in EXECUTOR_LIST:
                                    package_name = ILIB_PLUGINS_EXECUTOR_PKG
                                    logger.info(f"Feature {feature_name} found in package {package_name}")
                                else:
                                    #abstract feature likelihood
                                    package_name = feature_name
                                    logger.info(f"Feature {feature_name} found in package {package_name}")
                            else:
                                package_name = configurator.get_parent(feature_name)
                                #build a py path link to py package
                                logger.info(f"Feature {feature_name} found in package {package_name}")

                            pkg_share = get_package_share_directory(package_name)
                            
                            #check for CMakeLists.txt file
                            nav_to_package = os.path.join(pkg_share, '..', '..', '..', '..', 'src', project_name, package_name)

                            #confirm package existence
                            if not os.path.exists(nav_to_package) or not os.path.isdir(nav_to_package):
                                logger.info(f"Package {package_name} that should contain {feature_name} doesnot exist in project!")
                                return

                            cml_file_path = os.path.abspath(os.path.join(nav_to_package, 'CMakeLists.txt'))

                            #check for rclpy module
                            if package_name == ILIB_PLUGINS_DIRECTORY:
                                nav_to_rclpy_package = os.path.join(pkg_share, '..', '..', '..', '..', 'src', project_name, package_name, ILIB_PLUGINS_DIRECTORY)
                            else:
                                rclpy_plugin_dir_name = package_name
                                nav_to_rclpy_package = os.path.join(pkg_share, '..', '..', '..', '..', 'src', project_name, package_name, rclpy_plugin_dir_name)
                            
                            feature_filename = f"{feature_name}.py"
                            feature_module_path = os.path.abspath(os.path.join(nav_to_rclpy_package, feature_filename))

                            # check if the CMakeLists.txt file exists
                            if os.path.exists(cml_file_path):

                                #pascal case feature name joined with package name
                                comp_class_name = self.to_pascal_case(feature_name)
                                plugin_name = f"featx_plugin::{comp_class_name}"

                                comp_exists_result = subprocess.run(['ros2', 'component', 'list'],capture_output=True, text=True)

                                if any(f"/{feature_name}" in component for component in comp_exists_result.stdout.splitlines()):
                                    logger.info(f"FeatX plugin {feature_name} has already been loaded")
                                else:
                                    logger.info(f"Loading plugin {plugin_name} in package {package_name}")
                                    load_result = subprocess.run(['ros2', 'component', 'load', '/featx_container', package_name, plugin_name],capture_output=True, text=True)

                                    if load_result.returncode == 0:
                                        logger.info(load_result.stdout)
                                        configurator.updateConfigModelSelection(feature_name, True)
                                        
                                    elif load_result.returncode == 1:
                                        logger.info(load_result.stderr)

                                    logger.info(f"{feature_name} loaded successfully.\n")

                            #else if the rclpy module exists in py_plugin_executor
                            elif os.path.exists(feature_module_path):
                                logger.info(f"CMakeLists.txt file not found at: {cml_file_path}")
                                logger.info("Checking for rclpy package ...")
                                
                                #load python package of a separate thread
                                module_name = f"{package_name}.{feature_name}"
                                class_name = self.to_pascal_case(feature_name)
                                object_data = f"{{module_name: '{module_name}', class_name: '{class_name}'}}"
                                load_py_plugin = subprocess.run(['ros2', 'service', 'call', '/load_feature', 'featx_interfaces/srv/LoadFeature', object_data],capture_output=True, text=True)

                                if load_py_plugin.returncode == 0:
                                    match = re.search(r"message='([^']*)'", load_py_plugin.stdout)
                                    configurator.updateConfigModelSelection(feature_name, True)
                                    if match:
                                        message = match.group(1)
                                        logger.info(f"Service message: {message}")
                                    else:
                                        logger.info("Message not found in response.")
                                elif load_py_plugin.returncode == 1:
                                    logger.info(load_py_plugin.stderr)
                                
                            else:
                                logger.info(f"No rclcpp or rclpy package for the {feature_name} feature found.")

                    else:
                        logger.info("Binding time not late. Cannot run this command at compile time")

                except subprocess.CalledProcessError as e:
                    logger.info("Could not get live bindingTime value:")
                    logger.info(e.stderr)

        else:
            logger.info("Fix issue(s), build and rerun the command again")

    def check_running_component_list(self, feature_name):
        comp_result = subprocess.run(["ros2", "component", "list"], capture_output=True, text=True)

        if comp_result.returncode == 0:
            containers = {}
            current_container = None
            for line in comp_result.stdout.splitlines():
                if not line.startswith(" ") and line.strip() == "/featx_container":
                    logger.info(f"checking running component container {line.strip()}")
                    current_container = line.strip()
                    containers[current_container] = []

                if line.startswith(" ") and current_container == "/featx_container":
                    parts = line.strip().split()
                    if len(parts) >= 2:
                        component_id = parts[0]
                        component_name = " ".join(parts[1:])
                        containers[current_container].append((component_id, component_name))

            for comp_id, name in containers[current_container]:
                if feature_name == parts[1:]:
                    logger.info(f"Found! ID: {comp_id}, Name: {name}")
                    logger.info(f"The feature {name} you are attempting to select has been bound static and loaded at compile time.")
                    return "yes"
                
        return "no"
            
            
        
    def check_running_node_list(self, feature_name):
        result = subprocess.run(["ros2", "node", "list"], capture_output=True, text=True)
        output = result.stdout
        output_list = output.splitlines()

        if f"/{feature_name}" in output_list:
            logger.info(f"Found static feature running as node")
            return "yes"
        else:
            return "no"

    def to_pascal_case(self, name):
        parts = re.split(r'[\s_-]+', name)
        return ''.join(word.capitalize() for word in parts)
        
        
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
                self.run_ros2_featx_load_command(args.feature)
            else:
                logger.info(f"Configuration for {feature_exists} doesnot exist. If it is not an abstract feature, add a configuration in model/configs.json")
        else:
            logger.info(f"\t--- Feature({str(args.feature)}) has not been defined in features.json ---")


        
