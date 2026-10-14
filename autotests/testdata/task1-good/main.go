package main

func twoSum(nums []int, target int) []int {
	seen := map[int]int{}
	for i, value := range nums {
		if j, ok := seen[target-value]; ok {
			return []int{j, i}
		}
		seen[value] = i
	}
	return nil
}

func main() { _ = twoSum([]int{2, 7}, 9) }
