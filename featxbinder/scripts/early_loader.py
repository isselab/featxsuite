from featxcli.configurator import Configurator

template = """\
from launch import LaunchDescription
from launch_ros.actions import Node

def generate_launch_description():
    return LaunchDescription([
"""

#read configs and build node list
configurator = Configurator()
all_current_configs = configurator.readConfigs()


template += " ])"