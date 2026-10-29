package fixture

import "fmt"

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	fmt.Println(total)
	return total
}
