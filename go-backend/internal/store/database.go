package store

import (
	"encoding/json"
)

func GetList[T any](s *Store, key string) []T {
	if data := s.Read(key); data != nil {
		if list, ok := data.([]interface{}); ok {
			result := make([]T, 0, len(list))
			for _, item := range list {
				if obj, ok := item.(map[string]interface{}); ok {
					var t T
					if jsonData, err := json.Marshal(obj); err == nil {
						if err := json.Unmarshal(jsonData, &t); err == nil {
							result = append(result, t)
						}
					}
				}
			}
			return result
		}
	}
	return []T{}
}

func SaveList[T any](s *Store, key string, list []T) {
	s.Write(key, list)
}

func FindByName[T any](list []T, name string) *T {
	for _, item := range list {
		var itemName string
		if jsonData, err := json.Marshal(item); err == nil {
			var obj map[string]interface{}
			if err := json.Unmarshal(jsonData, &obj); err == nil {
				if n, ok := obj["name"].(string); ok {
					itemName = n
				}
			}
		}
		if itemName == name {
			copy := item
			return &copy
		}
	}
	return nil
}

func FindIndexByName[T any](list []T, name string) int {
	for i, item := range list {
		var itemName string
		if jsonData, err := json.Marshal(item); err == nil {
			var obj map[string]interface{}
			if err := json.Unmarshal(jsonData, &obj); err == nil {
				if n, ok := obj["name"].(string); ok {
					itemName = n
				}
			}
		}
		if itemName == name {
			return i
		}
	}
	return -1
}

func DeleteByName[T any](list *[]T, name string) bool {
	idx := FindIndexByName(*list, name)
	if idx >= 0 {
		*list = append((*list)[:idx], (*list)[idx+1:]...)
		return true
	}
	return false
}

func UpdateByName[T any](list []T, name string, newItem T) bool {
	idx := FindIndexByName(list, name)
	if idx >= 0 {
		list[idx] = newItem
		return true
	}
	return false
}

func InsertByPosition[T any](list *[]T, item T, position string) {
	if position == "top" {
		*list = append([]T{item}, *list...)
	} else {
		*list = append(*list, item)
	}
}
