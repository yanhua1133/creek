// 外部注解宏：没有任何定义且位于注解位置，整个替换为空格；表达式位置的同类名字不处理。
struct Buffer {
  char* data() EXT_LIFETIME_BOUND { return storage; }
  char storage[16];
};
int limit = EXT_MAX_SIZE;
