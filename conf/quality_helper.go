package conf

import (
	"strconv"
	"strings"
)

// ParseProxyIDSpec 解析逗号/区间形式的线路ID配置
func ParseProxyIDSpec(spec string) []int {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}

	var ids []int
	seen := make(map[int]struct{})
	parts := strings.Split(spec, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || start <= 0 || end <= 0 || end < start {
				continue
			}
			for i := start; i <= end; i++ {
				if _, exists := seen[i]; !exists {
					ids = append(ids, i)
					seen[i] = struct{}{}
				}
			}
			continue
		}

		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			continue
		}
		if _, exists := seen[id]; !exists {
			ids = append(ids, id)
			seen[id] = struct{}{}
		}
	}

	return ids
}

// FormatProxyIDs 将ID列表转为配置字符串
func FormatProxyIDs(ids []int) string {
	if len(ids) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, id := range ids {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Itoa(id))
	}
	return sb.String()
}
