"""Locating a feature's implementation through the ament resource index.

A feature is implemented by a component, and both component systems advertise
what they provide in the resource index: ``rclcpp_components`` for C++ and
``rclpy_components`` for Python. Each entry is ``class_name;target``, so asking
the index which package provides a feature replaces the older approach of
walking ``src/packages`` and guessing from the presence of a CMakeLists.txt.

That has three consequences worth knowing:

* featx no longer needs the source tree at runtime, only the install space.
* A feature's package is discovered rather than derived from its parent, so no
  hardcoded list of which features are Python is needed.
* A feature whose component was never registered is reported as unregistered,
  instead of failing later with a confusing path error.
"""

import re

from ament_index_python.resources import get_resource, get_resources

#: The containers featx loads components into.
CPP_CONTAINER = "/featx_container"
PY_CONTAINER = "/featx_py_container"

#: The resource index groups the two component systems register under.
CPP_GROUP = "rclcpp_components"
PY_GROUP = "rclpy_components"


class Ambiguous(Exception):
    """More than one package registers a component for the same feature."""


class Implementation:
    """Where a feature's component lives and how to load it."""

    def __init__(self, feature, package, plugin, container, language):
        self.feature = feature
        self.package = package
        self.plugin = plugin
        self.container = container
        self.language = language

    def __str__(self):
        return f"{self.plugin} in {self.package} ({self.language})"


def find(feature_name):
    """Return the component implementing a feature, or None if none does.

    Both registries are searched, so a feature is found wherever it lives
    without featx being told which language implements it.
    """
    wanted = to_pascal_case(feature_name)
    found = []

    for group, container, language in (
        (CPP_GROUP, CPP_CONTAINER, "rclcpp"),
        (PY_GROUP, PY_CONTAINER, "rclpy"),
    ):
        for package, plugin in _registered(group):
            if plugin.split("::")[-1] == wanted:
                found.append(Implementation(feature_name, package, plugin,
                                            container, language))

    if not found:
        return None

    if len(found) > 1:
        where = ", ".join(f"{i.package} ({i.language})" for i in found)
        raise Ambiguous(
            f"feature {feature_name!r} is provided by "
            f"more than one package: {where}"
        )

    return found[0]


def _registered(group):
    """Yield every (package, class name) pair registered under a group."""
    try:
        packages = get_resources(group)
    except Exception:  # noqa: BLE001 - an absent group is not an error
        return

    for package in sorted(packages):
        try:
            content, _ = get_resource(group, package)
        except Exception:  # noqa: BLE001 - skip a package we cannot read
            continue

        for line in content.splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue

            name, _, target = line.partition(";")
            if target:
                yield package, name.strip()


def to_pascal_case(name):
    """Convert a feature name to the class name convention components use."""
    return "".join(word.capitalize() for word in re.split(r"[\s_-]+", name))
