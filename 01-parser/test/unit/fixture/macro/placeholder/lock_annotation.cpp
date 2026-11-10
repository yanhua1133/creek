// 回归：声明符之后的函数式注解宏（abseil 的 ABSL_SHARED_LOCKS_REQUIRED）整个替换为空格，不得写成定义形态。
class Cache {
 public:
  int Size() const
      EXT_LOCKS_REQUIRED(mutex_)
  {
    return size_;
  }
  void Clear() EXT_LOCKS_EXCLUDED(mutex_);

 private:
  int size_;
};
