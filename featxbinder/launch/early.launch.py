import os
from ament_index_python.packages import get_package_share_directory
from launch import LaunchDescription
from launch_ros.actions import Node, ComposableNodeContainer
from launch_ros.descriptions import ComposableNode

def generate_launch_description():
    pkg_share = get_package_share_directory('featxbinder')
    yaml_file = os.path.join(pkg_share, 'config', 'featx_params.yaml')

    container = ComposableNodeContainer(
        name='featx_container',
        namespace='',
        package='rclcpp_components',
        executable='component_container_mt',
        composable_node_descriptions=[
            ComposableNode(
                package='featxbinder',
                plugin='featx_plugin::FeatxBinder',
                name='featx_binder',
                parameters=[yaml_file],
            ),
        ],
        output='screen',
    )
    return LaunchDescription([
    container,
    #add all static features to be loaded early here.
    Node(package='mapping', executable='mapping'),
    Node(package='featxcli', executable='plugin_registry') #donot remove this
 ])