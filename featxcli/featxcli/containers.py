import subprocess

from rclpy.logging import get_logger

logger = get_logger('containers')

CPP_CONTAINER = '/featx_container'
PY_CONTAINER = '/featx_py_container'


def find_component_id(container, feature_name):
    """Return the id of feature_name inside container, or None if it is not loaded there."""
    comp_result = subprocess.run(["ros2", "component", "list"], capture_output=True, text=True)

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
                logger.info(f"Found {feature_name} in {container} with ID: {parts[0]}")
                return parts[0]

    return None
