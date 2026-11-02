#define MODE 2

#if MODE == 1
int mode_one(void) { return 1; }
#elif MODE == 2
int mode_two(void) { return 2; }
#else
int mode_other(void) { return 0; }
#endif
