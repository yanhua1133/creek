#ifndef WIDGET_H
#define WIDGET_H

#include <string>

namespace ui {

// C++ 风格类，含命名空间与成员初始化
class Widget {
public:
    explicit Widget(std::string name) : name_(name) {}
    const std::string& name() const { return name_; }

private:
    std::string name_;
};

// 模板类，C grammar 无法正确解析
template <typename T>
class Box {
public:
    T value;
};

}  // namespace ui

#endif  // WIDGET_H
