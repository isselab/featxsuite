from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess

class LoadVerb(VerbExtension):
    """The verb for selecting a configured feature."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument('-f', '--feature', type=str, required=True, help='Loading a configured feature into a configuration')
        
    def run_ros2_featx_load_command(self):
        configurator = Configurator()
        configurator.checkRules()

    def check_if_feature_exists(self, feature_name):
        config = Configurator()
        return config.find_feature(feature_name)
    
    def main(self, *, args):
        
        feature_exists = self.check_if_feature_exists(args.feature)

        if feature_exists:
            print(f"++Loading feature: {str(args.feature)}...++")
            self.run_ros2_featx_load_command()
        else:
            print(f"++Feature: {str(args.feature)} has not been defined in features.json...++")


        
