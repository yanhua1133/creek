// 语句作用域中缺分号的语句宏：每行写成 NAME(); 占位。
#define HANDLE_KIND(KIND, value) case KIND: return value;
int handle(int kind) {
  switch (kind) {
    HANDLE_KIND(1, 10)
    HANDLE_KIND(2, 20)
    default:
      return 0;
  }
}
