// Copyright 2026 Jude Gyimah
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "rclcpp/rclcpp.hpp"

namespace featx_plugin{
    class FeatxBinder : public rclcpp::Node{
    public:
        FeatxBinder(const rclcpp::NodeOptions &options) : Node("featx_binder", options){
            this->declare_parameter<std::string>("bindingTime", "Early");
            rclcpp::Parameter param("bindingTime", "Late");
            this->set_parameter(param);
            RCLCPP_INFO(this->get_logger(), "featx_binder running...");
        }

        void update_parameter_before_shutdown(){
            rclcpp::Parameter param("bindingTime", "Early");
            this->set_parameter(param);
            RCLCPP_INFO(this->get_logger(), "bindingTime switched to Early before shutdown!");
        }
    };
}

#include "rclcpp_components/register_node_macro.hpp"
RCLCPP_COMPONENTS_REGISTER_NODE(featx_plugin::FeatxBinder)
