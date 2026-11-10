// 函数体内后跟代码块的宏：写成 NAME(); 占位，代码块仍是普通复合语句。
#define RUN_CASE(name) printf("case %s\n", name);
int printf(const char *fmt, ...);
int run_all(void) {
  RUN_CASE("first") {
    int value = 1;
    (void)value;
  }
  return 0;
}
