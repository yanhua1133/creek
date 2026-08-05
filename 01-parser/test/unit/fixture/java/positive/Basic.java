package demo;

// 演示用的简单类
public class Basic {
    // 返回两个整数之和
    public int add(int a, int b) {
        return a + b;
    }

    public static void main(String[] args) {
        Basic b = new Basic();
        System.out.println(b.add(1, 2));
    }
}
