# Copyright 2026 Jude Gyimah
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import subprocess

from featxcli.components import CPP_CONTAINER, PY_CONTAINER  # noqa: F401

from rclpy.logging import get_logger

logger = get_logger('containers')


def find_component_id(container, feature_name):
    """Return the id of feature_name in container, or None if not loaded."""
    comp_result = subprocess.run(["ros2", "component", "list"],
                                 capture_output=True, text=True)

    if comp_result.returncode != 0:
        logger.info(comp_result.stderr)
        return None

    in_container = False

    for line in comp_result.stdout.splitlines():
        #container names sit at column 0, their components are indented beneath
        if not line.startswith(" "):
            in_container = line.strip() == container
            continue

        if in_container:
            parts = line.strip().split()
            if len(parts) >= 2 and " ".join(parts[1:]) == f"/{feature_name}":
                logger.info(f"Found {feature_name} in {container} "
                            f"with ID: {parts[0]}")
                return parts[0]

    return None
