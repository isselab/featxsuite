from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess, re

configurator = Configurator()


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
                print("Configuration is not running. Run ros2 featx start_config")
            else:
                try:
                    bt_command = ["ros2", "param", "get", "/featx_binder", "bindingTime"]
                    bt_result = subprocess.run(bt_command, capture_output=True, text=True, check=True)
                    bt_value = bt_result.stdout.split(":")[1].strip()

                    if bt_value == "Late":
                        #run unload command
                        #cannot unload static feature
                        fmode = configurator.get_binding_mode(feature_name)

                        if fmode == "Static":
                            print("Warning: Cannot unload a static feature at runtime!")
                        else:
                            print(f"*** Unloading feature: {feature_name}...***")

                            #check component list
                            comp_result = subprocess.run(["ros2", "component", "list"], capture_output=True, text=True)

                            if comp_result.returncode == 0:
                                containers = {}
                                current_container = None
                                for line in comp_result.stdout.splitlines():
                                    if not line.startswith(" ") and line.strip() == "/featx_container":
                                        print("checking featx_container")
                                        current_container = line.strip()
                                        containers[current_container] = []

                                    if line.startswith(" ") and current_container == "/featx_container":
                                        parts = line.strip().split()
                                        if len(parts) >= 2:
                                            component_id = parts[0]
                                            component_name = " ".join(parts[1:])
                                            containers[current_container].append((component_id, component_name))
                                
                                found_as_component = False

                                for comp_id, name in containers[current_container]:
                                    if f"/{feature_name}" == name:
                                        print(f"Found! ID: {comp_id}, Name: {name}")
                                        found_as_component = True
                                        #run unload command
                                        unload_result = subprocess.run(["ros2", "component", "unload", "/featx_container", comp_id], capture_output=True, text=True)

                                        if unload_result.returncode == 0:
                                            print(unload_result.stdout)
                                            configurator.updateConfigModelSelection(feature_name, False)
                                        else:
                                            print(unload_result.stderr)

                                        break #break out of for loop

                                if not found_as_component:
                                    package_name = configurator.get_parent(feature_name)
                                    module_name = f"{package_name}.{feature_name}"
                                    
                                    object_data = f"{{module_name: '{module_name}'}}"
                                    unload_py_plugin = subprocess.run(['ros2', 'service', 'call', '/unload_feature', 'featx_interfaces/srv/UnloadFeature', object_data],capture_output=True, text=True)

                                    if unload_py_plugin.returncode == 0:
                                        match = re.search(r"message='([^']*)'", unload_py_plugin.stdout)
                                        configurator.updateConfigModelSelection(feature_name, False)
                                        if match:
                                            message = match.group(1)
                                            print(f"Service message: {message}")
                                        else:
                                            print("Message not found in response.")
                                    elif unload_py_plugin.returncode == 1:
                                        print(unload_py_plugin.stderr)


                            elif comp_result.returncode == 1:
                                print(comp_result.stderr)

                            print(f"{feature_name} unloaded successfully.")
                    else:
                        print("Binding time not late. Cannot run this command at compile time")

                except subprocess.CalledProcessError as e:
                    print("Could not get live bindingTime value:")
                    print(e.stderr)

        else:
            print("Fix issue(s), build and rerun the command again")

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
                self.run_ros2_featx_unload_command(args.feature)
            else:
                print(f"Configuration for {feature_exists} doesnot exist. If it is not an abstract feature, add a configuration in model/configs.json")
        else:
            print(f"\t--- Feature({str(args.feature)}) has not been defined in features.json ---")

