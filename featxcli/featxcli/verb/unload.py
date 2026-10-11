# Copyright 2026 Jude Gyimah
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess
from rclpy.logging import get_logger
from featxcli.containers import CPP_CONTAINER, PY_CONTAINER, find_component_id

configurator = Configurator()
logger = get_logger('unload')


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

    def shut_node_down(self, feature_name):
        """Unload a feature from whichever container is hosting it.

        Both languages are components, so there is one path: find the
        component by name and unload it by the id its container assigned.
        """
        logger.info(f"*** Unloading feature: {feature_name}...***")

        for container in (CPP_CONTAINER, PY_CONTAINER):
            comp_id = find_component_id(container, feature_name)

            if comp_id is None:
                continue

            result = subprocess.run(
                ["ros2", "component", "unload", container, comp_id],
                capture_output=True, text=True)

            if result.returncode == 0:
                logger.info(result.stdout.strip())
                configurator.updateConfigModelSelection(feature_name, False)
                logger.info(f"{feature_name} unloaded successfully.")
            else:
                logger.info(result.stderr.strip())

            return

        logger.info(f"{feature_name} is not loaded in {CPP_CONTAINER} or {PY_CONTAINER}")

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

