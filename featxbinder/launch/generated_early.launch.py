from launch import LaunchDescription
from launch_ros.actions import Node

def generate_launch_description():
    return LaunchDescription([
    Node(package='attach_service', executable='approach_service_server'),
    Node(package='mapping', executable='nav2_map_server'),
 ])