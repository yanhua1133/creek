// 错误紧跟在宏调用之后但与宏无关：放置占位符后错误不减少，撤销本轮，不留下占位符，错误保留。
int broken(void) {
  int value = COMPUTE(1, 2) + ;
  return value;
}
