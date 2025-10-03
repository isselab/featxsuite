import os, sys
import rclpy
from rclpy.node import Node
from rclpy.parameter import Parameter
from ros2cli.verb import VerbExtension
from launch import LaunchService, LaunchDescription
from launch.actions import IncludeLaunchDescription
from launch.launch_description_sources import PythonLaunchDescriptionSource
from ament_index_python.packages import get_package_share_directory


class StartConfigVerb(VerbExtension):
    """The verb for starting a configuration."""

    def add_arguments(self, parser, cli_name):
        pass

    def run_ros2_featx_start_config_command(self):

        pkg_share = get_package_share_directory('featxbinder')
        launch_file_path = os.path.join(pkg_share, 'launch', 'generated_early.launch.py')

        ls = LaunchService()

        ld = LaunchDescription([
            IncludeLaunchDescription(PythonLaunchDescriptionSource(launch_file_path))
        ])

        ls.include_launch_description(ld)

        return ls.run()
    
    def main(self, *, args):
        sys.exit(self.run_ros2_featx_start_config_command())
        node = Node("featx_binder")
        param = Parameter("bindingTime", Parameter.Type.STRING, "Late")
        node.set_parameters([param])

