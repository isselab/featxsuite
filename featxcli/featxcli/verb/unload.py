from ros2cli.verb import VerbExtension
from featxcli.configurator import Configurator
import subprocess


class UnloadVerb(VerbExtension):
    """The verb for deselecting a configured feature."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument('-f', '--feature', type=str, required=True, help='Unloading a configured feature into a configuration')

    def run_ros2_featx_unload_command(self, command):
        configurator = Configurator()
        configurator.checkRules()

    def check_if_feature_exists(self, feature_name):
        config = Configurator()
        return config.find_feature(feature_name)
    
    def main(self, *, args):
        feature_exists = self.check_if_feature_exists(args.feature)

        if feature_exists:
            print(f"++Loading feature: {str(args.feature)}...++")
            self.msgString = "{"+f"verb: 'unload', featureid: '{str(args.feature).strip()}'"+"}"
            self.command_input = ["ros2", "topic", "pub", "-1", "/fx_command", "featxcli_interfaces/msg/FxReconfigureCmd", self.msgString]
            self.run_ros2_featx_unload_command(self.command_input)
        else:
            print(f"++Feature: {str(args.feature)} has not been defined in features.json...++")

