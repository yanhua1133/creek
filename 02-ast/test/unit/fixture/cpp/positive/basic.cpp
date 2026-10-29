#include <string>

class Greeter {
public:
    std::string greet(const std::string &name) {
        if (!name.empty()) {
            return std::string("hello ") + name;
        }
        return "hello";
    }
};
