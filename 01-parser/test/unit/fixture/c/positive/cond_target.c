// 默认目标配置：Linux x86_64、GCC 13；Windows、macOS 和 clang 分支不保留。
#ifdef _WIN32
int platform = garbage windows {
#elif defined(__APPLE__)
int platform = garbage apple {
#elif defined(__linux__) && defined(__x86_64__)
int platform = 1;
#endif
#if __GNUC__ >= 4 && !defined(__clang__)
int gcc = 1;
#else
int gcc = garbage {
#endif
