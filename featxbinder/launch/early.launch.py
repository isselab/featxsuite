from launch import LaunchDescription
from launch_ros.actions import Node, ComposableNodeContainer
from launch_ros.descriptions import ComposableNode

def generate_launch_description():
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
            ),
        ],
        output='screen',
    )
    return LaunchDescription([
    container,
    #add all static features to be loaded early here.
    Node(package='navigation', executable='navigation'),
    Node(package='elevator', executable='elevator'),
    Node(package='footprint', executable='footprint'),
    Node(package='shelf', executable='shelf'),
    # The rclpy counterpart of featx_container: it hosts the Python features
    # so that both languages are loaded through `ros2 component`.
    Node(package='rclpy_components', executable='component_container_mt',
         name='featx_py_container') #donot remove this
 ])
