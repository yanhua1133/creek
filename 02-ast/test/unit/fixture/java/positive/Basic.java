package fixture;

import java.util.List;

class Basic {
    int sum(List<Integer> values) {
        int total = 0;
        for (int value : values) {
            total += value;
        }
        return total;
    }
}
