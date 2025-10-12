from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess, os, re
from ament_index_python.packages import get_package_share_directory


configurator = Configurator()

class LoadVerb(VerbExtension):
    """The verb for selecting a configured feature."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument('-f', '--feature', type=str, required=True, help='Loading a configured feature into a configuration')
        
    def run_ros2_featx_load_command(self, feature_name):
        configurator.checkRules()

        result = subprocess.run(["ros2", "node", "list"], capture_output=True, text=True)
        nodes = result.stdout.strip().split('\n')
        
        if len(nodes) == 1 and nodes[0] == "":
            print("Configuration is not running. Run ros2 featx start_config")
        else:
            try:
                bt_command = ["ros2", "param", "get", "/featx_binder", "bindingTime"]
                bt_result = subprocess.run(bt_command, capture_output=True, text=True, check=True)
                bt_value = bt_result.stdout.split(":")[1].strip()

                if bt_value == "Late":
                    
                    fmode = configurator.get_binding_mode(feature_name)
                    ftime = configurator.get_binding_time(feature_name)

                    if fmode == "Static" and ftime == "Early":
                        print("Warning: Cannot load a static early feature at runtime!")
                    else:
                        print(f"*** Loading feature: {feature_name}...***")
                        package_name = configurator.get_parent(feature_name)
                        pkg_share = get_package_share_directory(package_name)
                        
                        #check for CMakeLists.txt file
                        nav_to_package = os.path.join(pkg_share, '..', '..', '..', '..', 'src', 'packages', package_name)
                        cml_file_path = os.path.abspath(os.path.join(nav_to_package, 'CMakeLists.txt'))

                        #check for rclpy module
                        nav_to_rclpy_package = os.path.join(pkg_share, '..', '..', '..', '..', 'src', 'packages', package_name, package_name)
                        feature_filename = f"{feature_name}.py"
                        feature_module_path = os.path.abspath(os.path.join(nav_to_rclpy_package, feature_filename))
                        print(feature_module_path)

                        # check if the CMakeLists.txt file exists
                        if os.path.exists(cml_file_path):
                            
                            with open(cml_file_path, "r") as f:
                                content = f.read()

                            # Regular expression to match the target pattern
                            pattern = r'rclcpp_components_register_nodes\s*\([^)]*?"([^"]+)"\)'

                            match = re.search(pattern, content)
                            if match:
                                plugin_name = match.group(1)
                            else:
                                print("No registered component match found.")

                            print(f"Loading plugin {plugin_name} in package {package_name}")
                            load_result = subprocess.run(['ros2', 'component', 'load', '/featx_container', package_name, plugin_name],capture_output=True, text=True)

                            if load_result.returncode == 0:
                                print(load_result.stdout)
                                configurator.updateConfigModelSelection(feature_name, True)
                            elif load_result.returncode == 1:
                                print(load_result.stderr)

                            print(f"{feature_name} loaded successfully.\n")

                        #else if the rclpy module exists
                        elif os.path.exists(feature_module_path):
                            print(f"CMakeLists.txt file not found at: {cml_file_path}")
                            print("Checking for rclpy package ..")

                            #build class name from featurename
                            #load python package of a separate thread
                            class_name = ""
                            self.load_rclpy_plugin(feature_name, class_name)
                            
                        else:
                            print(f"No exacutable package for the {feature_name} feature found.")

                else:
                    print("Binding time not late. Cannot run this command at compile time")

            except subprocess.CalledProcessError as e:
                print("Could not get live bindingTime value:")
                print(e.stderr)

    def load_rclpy_plugin(self, module_name, class_name):
        pass
        
    def check_if_feature_exists(self, feature_name):
        return configurator.find_feature(feature_name)
    
    
    def check_if_config_exists(self, name_target):
        cparsed = configurator.readConfigs()
        match = next((cfg["name"] for cfg in cparsed["configs"] if cfg["name"] == name_target), None)
        return match

    
    def main(self, *, args):
        
        feature_exists = self.check_if_feature_exists(args.feature)

        if feature_exists:
            print(f"{feature_exists} feature exists")
            config_exists = self.check_if_config_exists(args.feature)

            if config_exists:
                print(f"{feature_exists} configuration exists\n")
                self.run_ros2_featx_load_command(args.feature)
            else:
                print(f"Configuration for {feature_exists} doesnot exist. If it is not an abstract feature, add a configuration in model/configs.json")
        else:
            print(f"\t--- Feature({str(args.feature)}) has not been defined in features.json ---")


        
