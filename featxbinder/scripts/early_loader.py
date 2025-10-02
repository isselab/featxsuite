import os
from featxcli.configurator import Configurator
import xml.etree.ElementTree as ET

template = """\
from launch import LaunchDescription
from launch_ros.actions import Node

def generate_launch_description():
    return LaunchDescription([
"""

#read configs and build node list
configurator = Configurator()
all_current_configs = configurator.readConfigs()
print("\nFeatures: ")

for config in all_current_configs['configs']:
   
    if config['bindingTime'] == "Early" and config['isSelected'] == True:
        #build unique path to package.xml file
        parent_feature_name = configurator.get_parent(config['name'])

        this_dir = os.path.dirname(os.path.abspath(__file__))
        package_xml_path = os.path.join(this_dir, f"../../../packages/{parent_feature_name}/package.xml")
        package_xml_path = os.path.normpath(package_xml_path)

        if os.path.exists(package_xml_path):
            tree = ET.parse(package_xml_path)
            root = tree.getroot()

            build_type_element = root.find("./export/build_type")

            if build_type_element is not None:

                #if its a c++ package
                if build_type_element.text == "ament_cmake":
                    
                    if parent_feature_name != None:
                        template += "    Node(package='"+ parent_feature_name +"', executable='"+config['name']+"'),\n"
                        print(f"\t{config['name']}")
                    else:
                        print("Parent feature not found")

                #if its a python package
                if build_type_element.text == "ament_python":

                    if parent_feature_name != None:
                        template += "    Node(package='"+ parent_feature_name +"', executable='"+config['name']+"'),\n"
                        print(f"\t{config['name']}")
                    else:
                        print("Parent feature not found")
            else:
                print("No <build_type> tag found")

        else:
            print(f"package.xml file for {config['name']} not found.")

    


template += " ])"

def create_launch_file(template_full):
    launch_dir = os.path.join(os.path.dirname(__file__), '..', 'launch')
    os.makedirs(launch_dir, exist_ok=True)
    
    launch_file_path = os.path.join(launch_dir, 'generated_early.launch.py')
    with open(launch_file_path, 'w') as f:
        f.write(template_full)


create_launch_file(template)
