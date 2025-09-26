from ros2cli.command import add_subparsers_on_demand
from ros2cli.command import CommandExtension

class FeatxCommand(CommandExtension):
    """featx command variable."""

    def add_arguments(self, parser, cli_name):
        self._subparser = parser
        # get verb extensions and let them add their arguments
        add_subparsers_on_demand(
            parser, cli_name, '_verb', 'featxcli.verb', required=False)

    def main(self, *, parser, args):
        if not hasattr(args, '_verb'):
            # in case no verb was passed
            self._subparser.print_help()
            return 0

        extension = getattr(args, '_verb')

        # call the verb's main method
        return extension.main(args=args)