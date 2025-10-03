from launch import LaunchDescription
from launch_ros.actions import Node

def generate_launch_description():
    return LaunchDescription([
    Node(package='attach_service', executable='approach_service_server'),
    Node(package='mapping', executable='nav2_map_server'),
    Node(
         package='featxbinder', 
         executable='featx_binder', 
         parameters=['/home/fr0b0/ros2_ws/install/featxbinder/share/featxbinder/config/featx_params.yaml'])
 ])