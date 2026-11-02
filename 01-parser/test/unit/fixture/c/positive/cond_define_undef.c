#define FEATURE_A
#undef FEATURE_A

#ifdef FEATURE_A
int with_a(void) { return 1; }
#else
int without_a(void) { return 0; }
#endif

#if 0
#define LEVEL 1
#endif

#if LEVEL != 1
int level_unknown(void) { return 2; }
#else
int level_one(void) { return 1; }
#endif
