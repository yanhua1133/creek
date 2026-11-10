// 没有语法错误的文件不放置占位符，合法的宏调用保持原样。
int check(int status) {
  CHECK_OK(status);
  return LOG_LEVEL(status);
}
