#include "rclcpp/rclcpp.hpp"

class FeatxBinder : public rclcpp::Node{
public:
    FeatxBinder() : Node("featx_binder"){
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

int main(int argc, char * argv[]){

    rclcpp::init(argc, argv);
    auto node = std::make_shared<FeatxBinder>();

    rclcpp::spin(node);

    node->update_parameter_before_shutdown();

    rclcpp::shutdown();
    return 0;
}
