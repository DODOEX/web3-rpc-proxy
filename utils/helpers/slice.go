package helpers

// 合并多个数组，并对相同 ID 的对象进行合并操作
func MergeSlicesBy[T comparable](iteratee func (a, b T) T, unique func (element T) string, arrays ...[]T) []T {
	// 创建一个 map 来存储已处理的对象，ID 为键
	exists := make(map[string]T)

	// 遍历所有数组
	for _, array := range arrays {
		for _, item := range array {
			// 检查该 ID 是否已存在于 map 中
			id := unique(item)
			if existing, found := exists[id]; found {
				// 如果已存在，则进行合并操作，例如合并 Value
				exists[id] = iteratee(existing, item)
			} else {
				// 如果不存在，直接添加到 map 中
				exists[id] = item
			}
		}
	}

	// 将合并后的 map 转换为 slice
	mergedArray := make([]T, 0, len(exists))
	for key := range exists {
		mergedArray = append(mergedArray, exists[key])
	}

	return mergedArray
}
