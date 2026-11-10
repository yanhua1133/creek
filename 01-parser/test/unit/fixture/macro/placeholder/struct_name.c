// 回归：struct 之后生成名字的宏（upb 的 UPB_PRIVATE）写成名字 NAME，不得写成定义形态。
typedef struct MANGLE(entry) {
  int value;
  const struct MANGLE(entry) *next;
} entry_t;
