#include "rclcpp/rclcpp.hpp"

class FeatxBinder : public rclcpp::Node{
public:
    FeatxBinder() : Node("featx_binder"){
        this->declare_parameter<std::string>("my_param", "default_value");
    }

    void update_parameter_before_shutdown(){
        rclcpp::Parameter param("my_param", "new_value_before_shutdown");
        this->set_parameter(param);
        RCLCPP_INFO(this->get_logger(), "Parameter updated just before shutdown!");
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
