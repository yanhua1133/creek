from typing import Iterable


class Calculator:
    def sum(self, values: Iterable[int]) -> int:
        total = 0
        for value in values:
            total += value
        if total > 0:
            print(total)
        return total
