from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess

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
                    #run load command
                    pass
                    print(f"{feature_name} loaded successfully.")
                else:
                    print("Binding time not late. Cannot run this command at compile time")

            except subprocess.CalledProcessError as e:
                print("Could not get live bindingTime value:")
                print(e.stderr)
        
    def check_if_feature_exists(self, feature_name):
        return configurator.find_feature(feature_name)
    
    def main(self, *, args):
        
        feature_exists = self.check_if_feature_exists(args.feature)

        if feature_exists:
            print(f"++Loading feature: {str(args.feature)}...++")
            self.run_ros2_featx_load_command(args.feature)
        else:
            print(f"++Feature: {str(args.feature)} has not been defined in features.json...++")


        
