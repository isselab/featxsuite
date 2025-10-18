from setuptools import find_packages, setup

package_name = 'featxcli'

setup(
    name=package_name,
    version='0.0.1',
    packages=find_packages(exclude=['test']),
    package_data={
        package_name: ['model/*.json'],  # <-- This line ensures model and config files get installed
    },
    data_files=[
        ('share/ament_index/resource_index/packages',
            ['resource/' + package_name]),
        ('share/' + package_name, ['package.xml']),
    ],
    install_requires=['setuptools', 'ros2cli'],
    extras_require={
    'test': ['pytest'],},
    zip_safe=True,
    maintainer='kofi',
    maintainer_email='',
    url='',
    download_url='',
    keywords=[],
    classifiers=[
        'Environment :: Console',
        'Intended Audience :: Developers, Operators, Manufacturers, System Engineers',
        'License :: OSI Approved :: Apache Software License',
        'Programming Language :: Python',
    ],
    description='A lightweight ROS 2 command line tool which extends feature models to support flexible feature binding.',
    license='MIT',
    tests_require=['pytest'],
    entry_points={
        'console_scripts': [
            'configurator = featxcli.configurator:main',
            'plugin_registry = featxcli.plugin_registry:main',
        ],
        'ros2cli.command': [
            'featx = featxcli.command.featx:FeatxCommand',
        ],
        'ros2cli.extension_point': [
            'ros2cli.verb = ros2cli.verb:VerbExtension',
        ],
        'featxcli.verb': [
            'load = featxcli.verb.load:LoadVerb',
            'unload = featxcli.verb.unload:UnloadVerb',
            'start_config = featxcli.verb.start_config:StartConfigVerb',
        ],
    },
)
