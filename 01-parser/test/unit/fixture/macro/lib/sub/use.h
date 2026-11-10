// 头文件：属性宏、函数式属性宏跨行调用、嵌套宏、条件未知时的候选定义、距离相同的定义和成员访问。
struct lib_state {
  int count;
  LIB_TSAN_MEMBER
};
LIB_API int lib_count(const char* LIB_NULLABLE name);
LIB_INLINE int lib_twice(int x) { return x * 2; }
LIB_PRINTF(1,
           2) void lib_log(const char* fmt, ...);
LIB_TIE int lib_tie_value;
static int lib_read(struct lib_state s) { return s.LIB_NULLABLE + s.count; }
