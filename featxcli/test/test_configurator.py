#!/usr/bin/env python3
import json
import os
import tempfile
from unittest.mock import mock_open, patch

import pytest

from featxcli.configurator import Configurator


class TestConfigurator:
    @pytest.fixture
    def temp_model_dir(self):
        """Create a temporary directory structure for testing"""
        with tempfile.TemporaryDirectory() as temp_dir:
            model_dir = os.path.join(temp_dir, "model")
            os.makedirs(model_dir)
            yield temp_dir, model_dir

    @pytest.fixture
    def sample_features_data(self):
        """Sample features.json data for testing"""
        return {
            "name": "root",
            "sub": [
                {
                    "name": "FeatureA",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "",
                    "sub": [
                        {
                            "name": "FeatureA1",
                            "bindingTimeAllowed": "Late",
                            "bindingModeAllowed": "Dynamic",
                            "excludes": "",
                        }
                    ],
                },
                {
                    "name": "FeatureB",
                    "bindingTimeAllowed": "Late",
                    "bindingModeAllowed": "Dynamic",
                    "excludes": "FeatureC",
                },
                {
                    "name": "FeatureC",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "",
                },
            ],
        }

    @pytest.fixture
    def sample_configs_data(self):
        """Sample configs.json data for testing"""
        return {
            "configs": [
                {
                    "name": "FeatureA",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
                {
                    "name": "FeatureA1",
                    "isSelected": True,
                    "bindingTime": "Late",
                    "bindingMode": "Dynamic",
                },
                {
                    "name": "FeatureB",
                    "isSelected": False,
                    "bindingTime": "Late",
                    "bindingMode": "Dynamic",
                },
                {
                    "name": "FeatureC",
                    "isSelected": False,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
            ]
        }

    def test_init(self):
        """Test Configurator initialization"""
        configurator = Configurator()

        assert hasattr(configurator, "base_dir")
        assert hasattr(configurator, "model_dir")
        assert hasattr(configurator, "all_configs")
        assert hasattr(configurator, "parent_feature")
        assert configurator.all_configs == ""
        assert configurator.parent_feature == ""

    def test_readFeatures_success(self, temp_model_dir, sample_features_data):
        """Test successful reading of features.json"""
        temp_dir, model_dir = temp_model_dir
        features_path = os.path.join(model_dir, "features.json")

        # Write sample data to features.json
        with open(features_path, "w") as f:
            json.dump(sample_features_data, f)

        # Mock the base_dir and model_dir
        with patch.object(Configurator, "__init__", lambda x: None):
            configurator = Configurator()
            configurator.base_dir = temp_dir
            configurator.model_dir = model_dir
            configurator.all_configs = ""
            configurator.parent_feature = ""

            result = configurator.readFeatures()
            assert result == sample_features_data

    def test_readFeatures_file_not_found(self, temp_model_dir, capsys):
        """Test readFeatures when features.json doesn't exist"""
        temp_dir, model_dir = temp_model_dir

        with patch.object(Configurator, "__init__", lambda x: None):
            configurator = Configurator()
            configurator.base_dir = temp_dir
            configurator.model_dir = model_dir
            configurator.all_configs = ""
            configurator.parent_feature = ""

            result = configurator.readFeatures()
            captured = capsys.readouterr()

            assert result is None
            assert "Error: Could not find features file" in captured.out

    def test_readFeatures_invalid_json(self, temp_model_dir, capsys):
        """Test readFeatures with invalid JSON"""
        temp_dir, model_dir = temp_model_dir
        features_path = os.path.join(model_dir, "features.json")

        # Write invalid JSON
        with open(features_path, "w") as f:
            f.write("invalid json {")

        with patch.object(Configurator, "__init__", lambda x: None):
            configurator = Configurator()
            configurator.base_dir = temp_dir
            configurator.model_dir = model_dir
            configurator.all_configs = ""
            configurator.parent_feature = ""

            result = configurator.readFeatures()
            captured = capsys.readouterr()

            assert result is None
            assert "Error: Failed to decode JSON" in captured.out

    def test_readConfigs_success(self, temp_model_dir, sample_configs_data):
        """Test successful reading of configs.json"""
        temp_dir, model_dir = temp_model_dir
        configs_path = os.path.join(model_dir, "configs.json")

        # Write sample data to configs.json
        with open(configs_path, "w") as f:
            json.dump(sample_configs_data, f)

        with patch.object(Configurator, "__init__", lambda x: None):
            configurator = Configurator()
            configurator.base_dir = temp_dir
            configurator.model_dir = model_dir
            configurator.all_configs = ""
            configurator.parent_feature = ""

            result = configurator.readConfigs()
            assert result == sample_configs_data

    def test_readConfigs_file_not_found(self, temp_model_dir, capsys):
        """Test readConfigs when configs.json doesn't exist"""
        temp_dir, model_dir = temp_model_dir

        with patch.object(Configurator, "__init__", lambda x: None):
            configurator = Configurator()
            configurator.base_dir = temp_dir
            configurator.model_dir = model_dir
            configurator.all_configs = ""
            configurator.parent_feature = ""

            result = configurator.readConfigs()
            captured = capsys.readouterr()

            assert result is None
            assert "Error: Could not find configs file" in captured.out

    def test_get_is_selected(self, sample_configs_data):
        """Test get_is_selected method"""
        configurator = Configurator()
        configurator.all_configs = sample_configs_data

        assert configurator.get_is_selected("FeatureA") is True
        assert configurator.get_is_selected("FeatureB") is False
        assert configurator.get_is_selected("NonExistent") is None

    def test_get_binding_time(self, sample_configs_data):
        """Test get_binding_time method"""
        configurator = Configurator()
        configurator.all_configs = sample_configs_data

        assert configurator.get_binding_time("FeatureA") == "Early"
        assert configurator.get_binding_time("FeatureB") == "Late"
        assert configurator.get_binding_time("NonExistent") is None

    def test_get_binding_mode(self, sample_configs_data):
        """Test get_binding_mode method"""
        configurator = Configurator()
        configurator.all_configs = sample_configs_data

        assert configurator.get_binding_mode("FeatureA") == "Static"
        assert configurator.get_binding_mode("FeatureB") == "Dynamic"
        assert configurator.get_binding_mode("NonExistent") is None

    def test_traverseModel_binding_inconsistency(
            self, sample_configs_data, capsys):
        """Test traverseModel detects binding inconsistencies"""
        configurator = Configurator()
        configurator.all_configs = sample_configs_data
        configurator.parent_feature = ""

        # Create a feature model with inconsistent bindings
        fx_model = {
            "name": "root",
            "sub": [
                {
                    "name": "FeatureA",
                    # Different from config (Early)
                    "bindingTimeAllowed": "Late",
                    # Different from config (Static)
                    "bindingModeAllowed": "Dynamic",
                    "excludes": "",
                }
            ],
        }

        configurator.traverseModel(fx_model)
        captured = capsys.readouterr()

        assert " *Feature (FeatureA): bindings not consistent" in captured.out

    def test_traverseModel_static_depends_on_dynamic(
            self, sample_configs_data, capsys):
        """Test traverseModel detects static child depending on dynamic parent"""
        # Modify configs to create the problematic scenario
        configs_with_issue = {
            "configs": [
                {
                    "name": "ParentFeature",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Dynamic",
                },
                {
                    "name": "ChildFeature",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
            ]
        }

        configurator = Configurator()
        configurator.all_configs = configs_with_issue
        configurator.parent_feature = ""

        fx_model = {
            "name": "ParentFeature",
            "sub": [
                {
                    "name": "ChildFeature",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "",
                }
            ],
        }

        configurator.traverseModel(fx_model)
        captured = capsys.readouterr()

        assert (
            " *Static and selected child ChildFeature depends on dynamic parent ParentFeature"
            in captured.out
        )

    def test_traverseModel_early_depends_on_late(self, capsys):
        """Test traverseModel detects early child depending on late parent"""
        configs_with_issue = {
            "configs": [
                {
                    "name": "ParentFeature",
                    "isSelected": False,
                    "bindingTime": "Late",
                    "bindingMode": "Dynamic",
                },
                {
                    "name": "ChildFeature",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
            ]
        }

        configurator = Configurator()
        configurator.all_configs = configs_with_issue
        configurator.parent_feature = ""

        fx_model = {
            "name": "ParentFeature",
            "sub": [
                {
                    "name": "ChildFeature",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "",
                }
            ],
        }

        configurator.traverseModel(fx_model)
        captured = capsys.readouterr()

        assert (
            " *Early child feature ChildFeature cannot depends on late parent ParentFeature"
            in captured.out
        )

    def test_traverseModel_mutual_exclusion(self, capsys):
        """Test traverseModel detects mutually exclusive features both selected"""
        configs_with_issue = {
            "configs": [
                {
                    "name": "FeatureA",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
                {
                    "name": "FeatureB",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
            ]
        }

        configurator = Configurator()
        configurator.all_configs = configs_with_issue
        configurator.parent_feature = ""

        fx_model = {
            "name": "root",
            "sub": [
                {
                    "name": "FeatureA",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "FeatureB",
                }
            ],
        }

        configurator.traverseModel(fx_model)
        captured = capsys.readouterr()

        assert (
            " *FeatureB and FeatureA exclude each other. They cannot be selected simultaneously"
            in captured.out
        )

    def test_traverseModel_selected_child_deselected_parent(self, capsys):
        """Test traverseModel detects selected child with deselected parent"""
        configs_with_issue = {
            "configs": [
                {
                    "name": "ParentFeature",
                    "isSelected": False,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
                {
                    "name": "ChildFeature",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                },
            ]
        }

        configurator = Configurator()
        configurator.all_configs = configs_with_issue
        configurator.parent_feature = ""

        fx_model = {
            "name": "ParentFeature",
            "sub": [
                {
                    "name": "ChildFeature",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "",
                }
            ],
        }

        configurator.traverseModel(fx_model)
        captured = capsys.readouterr()

        assert (
            " *Parent ParentFeature is deselected but child ChildFeature is selected"
            in captured.out
        )

    @patch.object(Configurator, "readFeatures")
    @patch.object(Configurator, "readConfigs")
    @patch.object(Configurator, "traverseModel")
    def test_checkRules_integration(
        self, mock_traverse, mock_read_configs, mock_read_features
    ):
        """Test checkRules method calls all necessary components"""
        mock_features = {"name": "root", "sub": []}
        mock_configs = {"configs": []}

        mock_read_features.return_value = mock_features
        mock_read_configs.return_value = mock_configs

        configurator = Configurator()
        configurator.checkRules()

        mock_read_features.assert_called_once()
        mock_read_configs.assert_called_once()
        mock_traverse.assert_called_once_with(mock_features)
        assert configurator.all_configs == mock_configs

    def test_full_integration_valid_model(
        self, temp_model_dir, sample_features_data, sample_configs_data
    ):
        """Test full integration with valid feature model"""
        temp_dir, model_dir = temp_model_dir

        # Write test files
        with open(os.path.join(model_dir, "features.json"), "w") as f:
            json.dump(sample_features_data, f)
        with open(os.path.join(model_dir, "configs.json"), "w") as f:
            json.dump(sample_configs_data, f)

        # Mock the initialization to use our temp directory
        with patch.object(Configurator, "__init__", lambda x: None):
            configurator = Configurator()
            configurator.base_dir = temp_dir
            configurator.model_dir = model_dir
            configurator.all_configs = ""
            configurator.parent_feature = ""

            # Should not raise any exceptions or print errors for valid model
            configurator.checkRules()

    def test_empty_excludes_handling(self):
        """Test that empty excludes field is handled properly"""
        configs = {
            "configs": [
                {
                    "name": "FeatureA",
                    "isSelected": True,
                    "bindingTime": "Early",
                    "bindingMode": "Static",
                }
            ]
        }

        configurator = Configurator()
        configurator.all_configs = configs
        configurator.parent_feature = ""

        fx_model = {
            "name": "root",
            "sub": [
                {
                    "name": "FeatureA",
                    "bindingTimeAllowed": "Early",
                    "bindingModeAllowed": "Static",
                    "excludes": "",  # Empty excludes should not cause issues
                }
            ],
        }

        # Should not raise any exceptions
        configurator.traverseModel(fx_model)

    def test_check_for_feature_found_top_level(self, sample_features_data):
        """Test finding a top-level feature"""
        configurator = Configurator()
        result = configurator.check_for_feature(
            sample_features_data, "FeatureA")
        assert result == "FeatureA"

    def test_check_for_feature_found_nested(self, sample_features_data):
        """Test finding a deeply nested feature"""
        configurator = Configurator()
        result = configurator.check_for_feature(
            sample_features_data, "FeatureA1")
        assert result == "FeatureA1"

    def test_check_for_feature_not_found(self, sample_features_data):
        """Test feature that does not exist"""
        configurator = Configurator()
        result = configurator.check_for_feature(
            sample_features_data, "NonExistentFeature"
        )
        assert result is None

    def test_check_for_feature_root(self, sample_features_data):
        """Test finding the root feature itself"""
        configurator = Configurator()
        result = configurator.check_for_feature(sample_features_data, "root")
        assert result == "root"
