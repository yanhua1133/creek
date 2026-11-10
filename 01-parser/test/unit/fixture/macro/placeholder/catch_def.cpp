// 声明作用域的定义型宏：TEST_CASE("...", "...") { 写成 void TEST_CASE() 占位；函数体内合法的宏调用不放置。
namespace demo {
TEST_CASE("parses macro", "[parser]") {
  int value = 1;
  REQUIRE(value == 1);
}
}  // namespace demo
