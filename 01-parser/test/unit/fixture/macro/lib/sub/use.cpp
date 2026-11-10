// C++ 文件：说明符关键字宏、放不下的展开、属性函数式宏和限定名。
class LIB_EXPORT Widget LIB_FINAL {
 public:
  LIB_CONSTEXPR_STATIC int kSize = 4;
  LS int kLimit = 8;
  LIB_DEPRECATED("use size") int old_size() const;
  virtual void run() LIB_FINAL;
};
LIB_INLINE int cpp_twice(int x) { return x * 2; }
int cpp_qualified = Widget::LIB_VALUE;
