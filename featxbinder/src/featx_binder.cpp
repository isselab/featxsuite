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
