from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess
from rclpy.logging import get_logger
from featxcli import components
from featxcli.containers import CPP_CONTAINER, PY_CONTAINER, find_component_id


configurator = Configurator()
logger = get_logger('load')

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

                            already_bound = (
                                find_component_id(CPP_CONTAINER, feature_name) is not None
                                or find_component_id(PY_CONTAINER, feature_name) is not None
                                or self.check_running_node_list(feature_name) == "yes")

                            if already_bound:
                                logger.info(f"The feature {feature_name} you are attempting to select has been bound static and loaded at compile time.")
                            else:
                                logger.info(f"Warning: Cannot load a static early feature({feature_name}) at runtime!")

                        else:
                            logger.info(f"*** Loading feature: {feature_name}...***")
                            self.load_component(feature_name)

                    else:
                        logger.info("Binding time not late. Cannot run this command at compile time")

                except subprocess.CalledProcessError as e:
                    logger.info("Could not get live bindingTime value:")
                    logger.info(e.stderr)

        else:
            logger.info("Fix issue(s), build and rerun the command again")

    def load_component(self, feature_name):
        """Load the component implementing a feature into its container.

        Both languages take the same path: the resource index says which
        package provides the feature and which container hosts it, and
        `ros2 component load` does the rest.
        """
        try:
            impl = components.find(feature_name)
        except components.Ambiguous as e:
            logger.info(str(e))
            return

        if impl is None:
            logger.info(
                f"No component is registered for feature {feature_name}. An rclcpp feature "
                f"registers with rclcpp_components_register_nodes, an rclpy one with "
                f"rclpy_components.setup_helper.register_nodes."
            )
            return

        if find_component_id(impl.container, feature_name) is not None:
            logger.info(f"FeatX plugin {feature_name} has already been loaded")
            return

        logger.info(f"Loading {impl} into {impl.container}")

        result = subprocess.run(
            ['ros2', 'component', 'load', impl.container, impl.package, impl.plugin],
            capture_output=True, text=True)

        if result.returncode == 0:
            logger.info(result.stdout.strip())
            configurator.updateConfigModelSelection(feature_name, True)
            logger.info(f"{feature_name} loaded successfully.\n")
        else:
            logger.info(result.stderr.strip())

    def check_running_node_list(self, feature_name):
        result = subprocess.run(["ros2", "node", "list"], capture_output=True, text=True)
        output = result.stdout
        output_list = output.splitlines()

        if f"/{feature_name}" in output_list:
            logger.info(f"Found static feature running as node")
            return "yes"
        else:
            return "no"

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


        
