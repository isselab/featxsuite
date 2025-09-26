cmake_minimum_required(VERSION 3.8)
project(featxbinder)

if(CMAKE_COMPILER_IS_GNUCXX OR CMAKE_CXX_COMPILER_ID MATCHES "Clang")
  add_compile_options(-Wall -Wextra -Wpedantic)
endif()

# find dependencies
find_package(ament_cmake REQUIRED)
find_package(rclcpp REQUIRED)
find_package(std_msgs REQUIRED)

file(READ "${CMAKE_CURRENT_SOURCE_DIR}/../featxcli/featxcli/model/configs.json" CONFIGS_JSON_STRING)

# Get number of elements in "configs"
string(JSON CONFIG_COUNT LENGTH ${CONFIGS_JSON_STRING} configs)

math(EXPR CONFIG_LAST_INDEX "${CONFIG_COUNT} - 1")

# Loop through configs
foreach(INDEX RANGE ${CONFIG_LAST_INDEX})
    # Extract the object at this index
    string(JSON CONFIG_OBJECT GET ${CONFIGS_JSON_STRING} configs ${INDEX})

    # Extract individual fields from this object
    string(JSON NAME GET ${CONFIG_OBJECT} name)
    string(JSON IS_SELECTED GET ${CONFIG_OBJECT} isSelected)
    string(JSON BINDING_TIME GET ${CONFIG_OBJECT} bindingTime)
    string(JSON BINDING_MODE GET ${CONFIG_OBJECT} bindingMode)

    message(STATUS "Config #${INDEX}")
    message(STATUS "  Name: ${NAME}")
    message(STATUS "  Selected: ${IS_SELECTED}")
    message(STATUS "  Binding Time: ${BINDING_TIME}")
    message(STATUS "  Binding Mode: ${BINDING_MODE}")

    string(TOUPPER "${NAME}" NAME_UPPER)

    if(IS_SELECTED STREQUAL "ON")
        message(NOTICE "${NAME} is selected")  
        option(ENABLE_${NAME_UPPER} "Enable building of package ${NAME_UPPER}" ON)
    else()
        message(NOTICE "Skipping: ${NAME} not selected")
        option(ENABLE_${NAME_UPPER} "Disabling building of package ${NAME_UPPER}" OFF)
    endif()

endforeach()

if(BUILD_TESTING)
  find_package(ament_lint_auto REQUIRED)
  # the following line skips the linter which checks for copyrights
  # comment the line when a copyright and license is added to all source files
  set(ament_cmake_copyright_FOUND TRUE)
  # the following line skips cpplint (only works in a git repo)
  # comment the line when this package is in a git repo and when
  # a copyright and license is added to all source files
  set(ament_cmake_cpplint_FOUND TRUE)
  ament_lint_auto_find_test_dependencies()
endif()

ament_package()
