// Package cache provides Redis cache repository implementation.
package cache

import "fmt"

// FormatTemplateListKey formats the cache key for subscription message template list.
func FormatTemplateListKey(appID string) string {
	return fmt.Sprintf("wechat:template_list:%s", appID)
}
