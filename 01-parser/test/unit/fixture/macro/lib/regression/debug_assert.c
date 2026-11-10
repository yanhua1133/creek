// 回归：函数式宏的保留定义不能改写时，不得回退到 #else 中的空定义而删除调用（redis 的 debugServerAssert）。
void regression_debug(int x) {
  LIB_DEBUG_CHECK(x > 0);
}
