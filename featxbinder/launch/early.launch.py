"""Bring up the features the configuration binds early.

Which features those are is read from the configuration model rather than
listed here, so this file and `featx load` cannot disagree about what is
selected. Editing configs.json is enough to change what comes up.

Early and static means bound when the system starts, not compiled in: every
feature is built and installed either way, and the late ones are bound later
with `featx load`.
"""

import os

from ament_index_python.packages import PackageNotFoundError, get_package_prefix
from featxcli.configurator import Configurator
from launch import LaunchDescription
from launch_ros.actions import ComposableNodeContainer, Node
from launch_ros.descriptions import ComposableNode


def early_features():
    """The features the configuration selects for early binding."""
    configs = Configurator().readConfigs()

    if not configs:
        # readConfigs logs what went wrong. Failing here is deliberate:
        # bringing up nothing at all is worse than refusing to launch.
        raise RuntimeError(
            "could not read the configuration model, so the early features "
            "are unknown"
        )

    # A parameter has no binding of its own and so no bindingTime: it is
    # bound with the node it belongs to, never launched as one.
    return [
        config["name"]
        for config in configs["configs"]
        if config.get("bindingTime") == "Early" and config["isSelected"]
    ]


def check_bindable_early(feature):
    """Fail early, and legibly, for a feature that cannot be a standalone node.

    An early feature is a package of its own whose node is named after it.
    That holds for the groupings in the model, which are one package each,
    but not for a feature that only ships components: those are bound later
    into a container, and launching one as a node would fail inside ros2
    with nothing to say which feature was at fault.
    """
    try:
        prefix = get_package_prefix(feature)
    except PackageNotFoundError:
        raise RuntimeError(
            f"feature {feature!r} is selected for early binding, but no such "
            f"package is installed; only a feature that is its own package "
            f"can be bound early"
        ) from None

    candidates = (os.path.join(prefix, "lib", feature, feature),
                  os.path.join(prefix, "lib", feature))

    if not any(os.path.isfile(c) for c in candidates):
        raise RuntimeError(
            f"feature {feature!r} is selected for early binding, but package "
            f"{feature!r} installs no {feature!r} executable, so it has no "
            f"node to launch. A feature that only provides components is "
            f"bound late, with `featx load`"
        )

    return feature


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

    early = [
        Node(package=feature, executable=feature)
        for feature in map(check_bindable_early, early_features())
    ]

    return LaunchDescription([
        container,
        *early,
        # The rclpy counterpart of featx_container: it hosts the Python
        # features so that both languages are loaded through
        # `ros2 component`.
        Node(package='rclpy_components', executable='component_container_mt',
             name='featx_py_container')  # donot remove this
    ])
