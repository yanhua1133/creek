#if defined(UNKNOWN_FEATURE)
#if 0
int dead(void) { return 0; }
#else
int nested_kept(void) { return 1; }
#endif
#elif 1
int second(void) { return 2; }
#endif
