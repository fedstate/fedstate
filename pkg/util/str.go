package util

import "fmt"

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func AddIndexSuffix(name string, index int) string {
	return fmt.Sprintf("%s-%d", name, index)
}

func AddSuffix(a, b string) string {
	return fmt.Sprintf("%s-%s", a, b)
}
