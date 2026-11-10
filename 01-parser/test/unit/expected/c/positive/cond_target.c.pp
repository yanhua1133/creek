blanked_bytes 222
unterminated 0
branch line=2 keyword=ifdef value=false taken=false condition="_WIN32"
branch line=4 keyword=elif value=false taken=false condition="defined(__APPLE__)"
branch line=6 keyword=elif value=true taken=true condition="defined(__linux__) && defined(__x86_64__)"
branch line=9 keyword=if value=true taken=true condition="__GNUC__ >= 4 && !defined(__clang__)"
branch line=11 keyword=else value=true taken=false condition=""
