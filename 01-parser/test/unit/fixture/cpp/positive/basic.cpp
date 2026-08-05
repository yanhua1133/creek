#include <vector>

namespace demo {

// 简单的计数器类
class Counter {
public:
    // 递增并返回当前值
    int Increment() { return ++value_; }

private:
    int value_ = 0;
};

}  // namespace demo

// 返回两者中的较大值
template <typename T>
T Max(T a, T b) {
    return a > b ? a : b;
}

int main() {
    demo::Counter c;
    return Max(c.Increment(), 5);
}
