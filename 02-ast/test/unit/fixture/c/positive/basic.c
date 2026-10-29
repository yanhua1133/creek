#include <stdio.h>

int add(int left, int right) {
    int sum = left + right;
    if (sum > 0) {
        printf("%d", sum);
    }
    for (int index = 0; index < right; index++) {
        sum += index;
    }
    while (sum > 100) {
        sum--;
    }
    do {
        sum++;
    } while (sum < 0);
    return sum;
}
