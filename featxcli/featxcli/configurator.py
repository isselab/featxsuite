#!/usr/bin/env python3
import os
import json
import subprocess
from pprint import pprint
from rclpy.logging import get_logger

logger = get_logger('configurator')

class Configurator:

    def __init__(self) -> None:
        self.base_dir = os.path.dirname(os.path.abspath(__file__))
        self.model_dir = os.path.join(self.base_dir, "model")
        self.all_configs = ""
        self.parent_feature = ""
        self.issue_count = 0

    def readFeatures(self):
        features_path = os.path.join(self.model_dir, "features.json")
        try:
            with open(features_path) as featureSet:
                features_json_data = json.load(featureSet)
                return features_json_data
        except FileNotFoundError:
            logger.info(f"Error: Could not find features file at {features_path}")
        except json.JSONDecodeError:
            logger.info(f"Error: Failed to decode JSON in {features_path}")

    def readConfigs(self):
        configs_path = os.path.join(self.model_dir, "configs.json")
        try:
            with open(configs_path) as configFile:
                configs_json_data = json.load(configFile)
                return configs_json_data
        except FileNotFoundError:
            logger.info(f"Error: Could not find configs file at {configs_path}")
        except json.JSONDecodeError:
            logger.info(f"Error: Failed to decode JSON in {configs_path}")

    def get_is_selected(self, name):
        for cfg in self.all_configs['configs']:
            if cfg["name"] == name:
                return cfg["isSelected"]
        return None
    
    def check_for_feature(self, features, name):
        if features.get("name") == name:
            return features.get("name")  # Found the target
        
        #search recursively in "sub" nodes if present
        for child in features.get("sub", []):
            result = self.check_for_feature(child, name)
            if result:
                return result
        
        return None  #not found
    

    def find_feature(self, name):
        features = self.readFeatures()
        return self.check_for_feature(features, name)

    
    def get_binding_time(self, name):
        for cfg in self.all_configs['configs']:
            if cfg["name"] == name:
                return cfg["bindingTime"]
        return None
    
    def get_binding_mode(self, name):
        for cfg in self.all_configs['configs']:
            if cfg["name"] == name:
                return cfg["bindingMode"]
        return None
    
    def check_for_parent_feature(self, features, name):
        if "sub" in features:
            parent_feature_name = features['name']
            
            for subObject in features['sub']:
                
                if subObject['name'] == name:
                    return parent_feature_name
                else:
                    result = self.check_for_parent_feature(subObject, name)
                    if result:  #if found in recursion, bubble it up
                        return result
        return None
    
    def get_parent(self, config_name):
        features = self.readFeatures()
        return self.check_for_parent_feature(features, config_name)

    def traverseModel(self, fx_model):

        if "sub" in fx_model:
        
            if fx_model['name'] != "root":
                self.parent_feature = fx_model['name']

            for subObject in fx_model['sub']:
                #find config and compare bindings
                for config in self.all_configs['configs']:
                    if config['name'] == subObject['name']:
                        if config['bindingTime'] != subObject['bindingTimeAllowed'] or config['bindingMode'] != subObject['bindingModeAllowed']:
                            logger.info(f" *Feature ({subObject['name']}): bindings not consistent")
                            self.issue_count = self.issue_count + 1

                        #selected static cannot depend on dynamic
                        if config['bindingMode'] == "Static" and self.get_is_selected(subObject['name']) == True and self.get_binding_mode(self.parent_feature) == "Dynamic":
                            logger.info(f" *Static and selected child {subObject['name']} depends on dynamic parent {self.parent_feature}")
                            self.issue_count = self.issue_count + 1

                        #early feature cannot depend on a late feature unless none are selected or late is selected first
                        if self.get_binding_time(self.parent_feature) == "Late" and self.get_binding_time(subObject['name']) == "Early":
                            if self.get_is_selected(self.parent_feature) == False and self.get_is_selected(subObject['name']) == True:
                                logger.info(f" *Early child feature {subObject['name']} cannot depends on late parent {self.parent_feature}, unless none are selected or the late is selected first")
                                self.issue_count = self.issue_count + 1

                        #features that exclude each other cannot be select simultaneaosly
                        if subObject['excludes'] != "":
                            excluded = subObject['excludes']
                            excluding = subObject['name']

                            if self.get_is_selected(excluded) == True and self.get_is_selected(excluding) == True:
                                logger.info(f" *{excluded} and {excluding} exclude each other. They cannot be selected simultaneously")
                                self.issue_count = self.issue_count + 1

                #selected feature cannot depend on deselected one
                if self.get_is_selected(self.parent_feature) == False and self.get_is_selected(subObject['name']) == True:
                    logger.info(f" *Parent {self.parent_feature} is deselected but child {subObject['name']} is selected")
                    self.issue_count = self.issue_count + 1
                    self.parent_feature = ""

                          
                self.traverseModel(subObject)
                       

    def checkRules(self):
        all_features = self.readFeatures()
        self.all_configs = self.readConfigs()

        self.traverseModel(all_features)
        logger.info(f"\t--- ({self.issue_count}) issue(s) detected ---")
        return self.issue_count

    def updateConfigModelSelection(self, feature_name, selection_status):
        logger.info("Updating configuration ...")
        current_config =  self.readConfigs()
        for config in current_config['configs']:
            if config['name'] == feature_name:
                config['isSelected'] = selection_status
                configs_file_path = os.path.join(self.model_dir, "configs.json")
    
                with open(configs_file_path, "w") as f:
                    json.dump(current_config, f, indent=4)
                break

        logger.info("Updating dependencies ...")
        
        #do this if only a feature load/selection is triggered
        if selection_status is True:
            self.recursive_dependant_load(feature_name)
        
        logger.info("Configuration updated successfully")

    def recursive_dependant_load(self, feature_name):
        includes_value = self.find_feature_includes(feature_name)
        
        if isinstance(includes_value, str) and includes_value != "":
            #run featx command to load single dependant
            logger.info(f"Updating the following dependencies: {includes_value}")
            load_dependant_result = subprocess.run(['ros2', 'featx', 'load', '-f', includes_value.strip()],capture_output=True, text=True)

            if load_dependant_result.returncode == 0:
                logger.info(load_dependant_result.stdout)
                self.updateConfigModelSelection(includes_value, True)
                logger.info(f"Dependency {includes_value} selected")
            elif load_dependant_result.returncode == 1:
                logger.info(load_dependant_result.stderr)

        elif isinstance(includes_value, list):
            #run featx command to load multiple dependants
            logger.info(f"Updating the following dependencies: {includes_value}")

            for feature_value in includes_value:
                #check if parent is not root
                parent_name = self.get_parent(feature_value)
                logger.info(f"Dependant: {feature_value} ----- Parent: {parent_name}")

                if parent_name != "root":
                    load_dependant_parent_result = subprocess.run(['ros2', 'featx', 'load', '-f', parent_name.strip()],capture_output=True, text=True)
                    if load_dependant_parent_result.returncode == 0:
                        logger.info(load_dependant_parent_result.stdout)
                        logger.info(f"Parent {parent_name} of dependency {feature_value} selected")
                        load_dependant_multiple_result = subprocess.run(['ros2', 'featx', 'load', '-f', feature_value.strip()],capture_output=True, text=True)
                else:
                    load_dependant_multiple_result = subprocess.run(['ros2', 'featx', 'load', '-f', feature_value.strip()],capture_output=True, text=True)

                    if load_dependant_multiple_result.returncode == 0:
                        logger.info(load_dependant_multiple_result.stdout)
                        self.updateConfigModelSelection(feature_value, True)
                        logger.info(f"Dependency {feature_value} selected")
                    elif load_dependant_multiple_result.returncode == 1:
                        logger.info(load_dependant_multiple_result.stderr)
        

    def get_feature_includes(self, features, name):
        
        if features.get("name") == name:
            return features.get("includes")  # Found the target, return includes value
        
        #search recursively in "sub" nodes if present
        for child in features.get("sub", []):
            result = self.get_feature_includes(child, name)
            if result:
                return result
        
        return None  #not found
    
    def find_feature_includes(self, name):
        features = self.readFeatures()
        return self.get_feature_includes(features, name)