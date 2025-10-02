#!/usr/bin/env python3
import os
import json
from pprint import pprint

class Configurator:
    def __init__(self) -> None:
        self.base_dir = os.path.dirname(os.path.abspath(__file__))
        self.model_dir = os.path.join(self.base_dir, "model")
        self.all_configs = ""
        self.parent_feature = ""

    def readFeatures(self):
        features_path = os.path.join(self.model_dir, "features.json")
        try:
            with open(features_path) as featureSet:
                features_json_data = json.load(featureSet)
                #pprint(features_json_data)
                return features_json_data
        except FileNotFoundError:
            print(f"Error: Could not find features file at {features_path}")
        except json.JSONDecodeError:
            print(f"Error: Failed to decode JSON in {features_path}")

    def readConfigs(self):
        configs_path = os.path.join(self.model_dir, "configs.json")
        try:
            with open(configs_path) as configFile:
                configs_json_data = json.load(configFile)
                #pprint(configs_json_data)
                return configs_json_data
        except FileNotFoundError:
            print(f"Error: Could not find configs file at {configs_path}")
        except json.JSONDecodeError:
            print(f"Error: Failed to decode JSON in {configs_path}")

    def get_is_selected(self, name):
        for cfg in self.all_configs['configs']:
            if cfg["name"] == name:
                return cfg["isSelected"]
        return None
    
    def check_for_feature(self, features, name):
        if "sub" in features:
            for subObject in features['sub']:
                if subObject['name'] == name:
                    return True
                else:
                    self.check_for_feature(subObject, name)
        return None
    

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
                            print(f"Feature ({subObject['name']}) bindings not consistent")

                        #selected static cannot depend on dynamic
                        if config['bindingMode'] == "Static" and self.get_is_selected(subObject['name']) == True and self.get_binding_mode(self.parent_feature) == "Dynamic":
                            print(f"Static and selected child {subObject['name']} depends on dynamic parent {self.parent_feature}")

                        #early feature cannot depend on a late feature unless none are selected or late is selected first
                        if self.get_binding_time(self.parent_feature) == "Late" and self.get_binding_time(subObject['name']) == "Early":
                            if self.get_is_selected(self.parent_feature) == False and self.get_is_selected(subObject['name']) == True:
                                print(f"Early child feature {subObject['name']} cannot depends on late parent {self.parent_feature}, unless none are selected or the late is selected first")

                        #features that exclude each other cannot be select simultaneaosly
                        if subObject['excludes'] != "":
                            excluded = subObject['excludes']
                            excluding = subObject['name']

                            if self.get_is_selected(excluded) == True and self.get_is_selected(excluding) == True:
                                print(f"{excluded} and {excluding} exclude each other. They cannot be selected simultaneously")

                #selected feature cannot depend on deselected one
                if self.get_is_selected(self.parent_feature) == False and self.get_is_selected(subObject['name']) == True:
                    print(f"Parent {self.parent_feature} is deselected but child {subObject['name']} is selected")
                    self.parent_feature = ""

                          
                self.traverseModel(subObject)
                       

    def checkRules(self):
        all_features = self.readFeatures()
        self.all_configs = self.readConfigs()

        self.traverseModel(all_features)
    