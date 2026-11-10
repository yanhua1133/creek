// 本目录最近的宏定义：属性、说明符、函数式属性宏和各类不可改写的宏。
#ifdef _MSC_VER
#define LIB_EXPORT __declspec(dllexport)
#else
#define LIB_EXPORT __attribute__((visibility("default")))
#endif
#define LIB_NULLABLE
#define LIB_FINAL final
#ifdef __cplusplus
#define LIB_INLINE inline
#else
#define LIB_INLINE static __inline__
#endif
#define LIB_API LIB_EXPORT
#define LIB_PRINTF(fmt, first) __attribute__((format(printf, fmt, first)))
#if defined(LIB_HAVE_TSAN)
#define LIB_TSAN_MEMBER char tsan_race_ = 0;
#else
#define LIB_TSAN_MEMBER
#endif
#define LIB_CONSTEXPR_STATIC static constexpr
#define LS static constexpr
#define LIB_VALUE 42
#define LIB_LOOP_A LIB_LOOP_B
#define LIB_LOOP_B LIB_LOOP_A
#define LIB_TEMP_HELPER
#undef LIB_TEMP_HELPER
#define LIB_DEPRECATED(msg) [[deprecated(msg)]]
#define LIB_CHECK(cond) do { if (!(cond)) abort(); } while (0)
#define LIB_TIE volatile
#ifdef LIB_DEBUG_ASSERT
#define LIB_DEBUG_CHECK(cond) lib_assert(cond)
#else
#define LIB_DEBUG_CHECK(cond)
#endif
