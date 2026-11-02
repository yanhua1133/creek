const char* kTemplate = R"cc(
#ifdef NOT_A_DIRECTIVE
  value
#endif
)cc";

int after_raw() { return 0; }
