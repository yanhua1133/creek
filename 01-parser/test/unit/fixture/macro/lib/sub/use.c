// C 文件：可省略关键字、本文件定义优先与 #undef、实现文件定义不外泄、常量宏、循环宏和关键字同名宏。
LIB_INLINE int c_twice(int x) { return x + x; }
#define LIB_NULLABLE const
int* LIB_NULLABLE c_pointer;
#undef LIB_NULLABLE
int LIB_NULLABLE = 0;
int LIB_LEAK_FROM_C;
int LIB_TEMP_HELPER;
int c_value = LIB_VALUE;
int c_loop = LIB_LOOP_A;
inline int c_keyword(void) { return 0; }
void c_check(int x) { LIB_CHECK(x > 0); }
