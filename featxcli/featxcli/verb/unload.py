from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess

configurator = Configurator()


class UnloadVerb(VerbExtension):
    """The verb for deselecting a configured feature."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument('-f', '--feature', type=str, required=True, help='Unloading a configured feature from a configuration')

    def run_ros2_featx_unload_command(self):
        configurator.checkRules()
        #run ros2 unload command for plugins

    def check_if_feature_exists(self, feature_name):
        return configurator.find_feature(feature_name)
    
    def main(self, *, args):
        feature_exists = self.check_if_feature_exists(args.feature)

        if feature_exists:
            print(f"++Loading feature: {str(args.feature)}...++")
            self.run_ros2_featx_unload_command()
        else:
            print(f"++Feature: {str(args.feature)} has not been defined in features.json...++")

