// 回归：初始化列表中提供元素的宏（Python C API 的 PyVarObject_HEAD_INIT）后面紧跟操作数时写成 NAME(), 占位，不得写成语句。
struct type_object { int refcnt; void *type; int size; const char *name; int flags; };
static struct type_object demo_type = {
    HEAD_INIT(&base_type, 0)
    "demo.Type",
    0,
};
