// 实参不合法的表达式宏：写成 NAME() 占位；没有错误的合法宏调用不放置。
int total(void) {
  int size = FIELD_COUNT(struct point, int x; int y);
  return size + MAX_ITEMS(size, 4);
}
