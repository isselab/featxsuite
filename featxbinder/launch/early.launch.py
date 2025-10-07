from launch import LaunchDescription
from launch_ros.actions import Node

def generate_launch_description():
    return LaunchDescription([
    #add all static features to be loaded early here.
    Node(package='mapping', executable='mapping'),
    Node(
         package='featxbinder', 
         executable='featx_binder', 
         parameters=['/home/fr0b0/ros2_ws/install/featxbinder/share/featxbinder/config/featx_params.yaml'])
 ])