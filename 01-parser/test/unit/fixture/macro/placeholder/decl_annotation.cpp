// 回归：声明之前的函数式外部注解宏（abseil 的 ABSL_DEPRECATE_AND_INLINE、ABSL_DEPRECATED）整个替换为空格；写入文本与原文相同时不放置。
EXT_DEPRECATED("use Bar") int Foo();
EXT_INLINE_HINT() inline int Bar() { return 1; }
