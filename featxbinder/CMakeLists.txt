cmake_minimum_required(VERSION 3.8)
project(featxbinder)

if(CMAKE_COMPILER_IS_GNUCXX OR CMAKE_CXX_COMPILER_ID MATCHES "Clang")
  add_compile_options(-Wall -Wextra -Wpedantic)
endif()

# find dependencies
find_package(ament_cmake REQUIRED)
find_package(rclcpp REQUIRED)
find_package(std_msgs REQUIRED)
find_package(Python3 REQUIRED COMPONENTS Interpreter)

file(READ "${CMAKE_CURRENT_SOURCE_DIR}/../featxcli/featxcli/model/configs.json" CONFIGS_JSON_STRING)

# Get number of elements in "configs"
string(JSON CONFIG_COUNT LENGTH ${CONFIGS_JSON_STRING} configs)

math(EXPR CONFIG_LAST_INDEX "${CONFIG_COUNT} - 1")

set(counter 0)

# Loop through configs
foreach(INDEX RANGE ${CONFIG_LAST_INDEX})
    # Extract the object at this index
    string(JSON CONFIG_OBJECT GET ${CONFIGS_JSON_STRING} configs ${INDEX})

    # Extract individual fields from this object
    string(JSON NAME GET ${CONFIG_OBJECT} name)
    string(JSON IS_SELECTED GET ${CONFIG_OBJECT} isSelected)
    string(JSON BINDING_TIME GET ${CONFIG_OBJECT} bindingTime)
    string(JSON BINDING_MODE GET ${CONFIG_OBJECT} bindingMode)

    message(STATUS "  Name: ${NAME}")

    string(TOUPPER "${NAME}" NAME_UPPER)

    if(IS_SELECTED STREQUAL "ON")
        #message(NOTICE "${NAME} is selected")
        math(EXPR counter "${counter} + 1")
    endif()

endforeach()

message(NOTICE "Compiling ${counter} selected early feature(s)")

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

add_executable(featx_binder src/featx_binder.cpp)
ament_target_dependencies(featx_binder rclcpp)

install(TARGETS
featx_binder
DESTINATION lib/${PROJECT_NAME}
)

install(DIRECTORY
  launch
  config
  DESTINATION share/${PROJECT_NAME}
)

ament_package()
