"""模块级文档字符串。"""


def add(a, b):
    # 返回两数之和
    return a + b


class Counter:
    def __init__(self):
        self.value = 0

    def increment(self):
        self.value += 1
        return self.value


if __name__ == "__main__":
    print(add(1, 2))
