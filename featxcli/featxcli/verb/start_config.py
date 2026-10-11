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

import os
import subprocess
import sys

from ament_index_python.packages import get_package_share_directory
from launch import LaunchDescription, LaunchService
from launch.actions import IncludeLaunchDescription
from launch.launch_description_sources import PythonLaunchDescriptionSource
from rclpy.logging import get_logger
from ros2cli.verb import VerbExtension

from featxcli import static_binding
from featxcli.configurator import Configurator

configurator = Configurator()
logger = get_logger('start_config')


class StartConfigVerb(VerbExtension):
    """The verb for starting a configuration."""

    def add_arguments(self, parser, cli_name):
        parser.add_argument(
            '--workspace', type=str, default=None,
            help='the workspace holding the feature packages '
                 '(default: the nearest one enclosing the current directory)')
        parser.add_argument(
            '--no-build', action='store_true',
            help='generate the static binding header and launch without rebuilding')

    def bind_static_early(self, workspace, build):
        """Compile the selected static early features in, and leave the rest out.

        These features are bound at compile time, so the configuration has to
        reach the build before it can reach the running system: a feature that
        is not compiled in cannot be activated at runtime, and one that is no
        longer selected has to leave the binary. Returns False when the
        configuration cannot be delivered, in which case nothing is launched.
        """
        if workspace is None:
            workspace = static_binding.find_workspace()

            if workspace is None:
                logger.info(
                    f"No workspace found above {os.getcwd()}: looked for a src/ "
                    f"holding {static_binding.OWNER}. Pass --workspace.")
                return False

            logger.info(f"Workspace: {workspace}")

        header = static_binding.locate(workspace)

        if header is None:
            logger.info(
                f"Could not find {static_binding.OWNER}'s "
                f"{static_binding.HEADER} under {workspace}.")
            return False

        configs = configurator.readConfigs()
        chosen = static_binding.selected(configs)
        dropped = static_binding.deselected(configs)

        changed = static_binding.write(header, static_binding.header(chosen))

        logger.info(f"Static early features: {', '.join(chosen) if chosen else 'none'}")
        if dropped:
            logger.info(f"Left out of the build: {', '.join(dropped)}")

        if not build:
            if changed:
                logger.info(
                    "The static binding changed but --no-build was given, so the "
                    "binaries still carry the previous selection.")
            return True

        if not changed:
            logger.info("Static binding unchanged.")

        # featxbinder owns the header, so it is reinstalled before the
        # compositions that include it are compiled against it.
        packages = [static_binding.OWNER] + chosen

        logger.info(f"Building {' '.join(packages)} ...")

        build_result = subprocess.run(
            ['colcon', 'build', '--packages-select'] + packages,
            cwd=workspace)

        if build_result.returncode != 0:
            logger.info("Build failed, so the configuration was not started.")
            return False

        # colcon installs but never uninstalls, so a feature dropped from the
        # configuration keeps the binary it was built with. It would not be
        # launched, but it is still deployed and could still be started by
        # hand, which is what binding it statically was meant to rule out.
        stale = static_binding.still_installed(workspace, dropped)

        if stale:
            logger.info(
                f"Still installed from an earlier build, though no longer selected: "
                f"{', '.join(stale)}. Remove install/<feature> and build/<feature> "
                f"to take them out of the deployment.")

        return True

    def run_ros2_featx_start_config_command(self):
        logger.info("Launching early features that are selected...")

        pkg_share = get_package_share_directory('featxbinder')
        launch_file_path = os.path.join(pkg_share, 'launch', 'early.launch.py')

        ls = LaunchService()

        ld = LaunchDescription([
            IncludeLaunchDescription(PythonLaunchDescriptionSource(launch_file_path))
        ])

        ls.include_launch_description(ld)

        return ls.run()

    def main(self, *, args):
        conflicts = configurator.checkRules()

        if conflicts != 0:
            logger.info("Fix issue(s), build and rerun the command again")
            return

        if not self.bind_static_early(args.workspace, not args.no_build):
            return

        sys.exit(self.run_ros2_featx_start_config_command())
