package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	unknownContentVersion      = "unknown"
	contentVersionPrefix       = "sha256:"
	contentVersionHashHexLen   = sha256.Size * 2
	generatedContentVersionLen = len(contentVersionPrefix) + contentVersionHashHexLen
	maxContentVersionLen       = 96
)

/*
 * contentVersion 是内容包指纹，不是人工维护的版本号。这里的“指纹”可以理解成：
 * 服务端把本次返回给前端的配置、商品、场景、事件、终局、状态和音轨入口整理成一段 JSON，
 * 再用 SHA-256 算法得到一个稳定字符串。只要内容包有任何实际变化，字符串就会变化；
 * 如果内容完全相同，多次启动服务得到的字符串也应该相同。
 *
 * 这个字段会跟随 /api/content/bootstrap 返回给前端，前端一局结束后再原样提交回 /api/runs。
 * 数据库把它保存到 runs.content_version。排行榜暂时不展示它，因为玩家不需要看到这串
 * 技术字符串；但开发调金额、换 seed 或排查旧成绩时，可以靠它判断某条成绩使用的是哪
 * 一版内容包，避免新旧平衡规则混在一起看。
 */
func withContentVersion(bootstrap bootstrapResponse) bootstrapResponse {
	bootstrap.Config.ContentVersion = calculateContentVersion(bootstrap)
	return bootstrap
}

func calculateContentVersion(bootstrap bootstrapResponse) string {
	bootstrap.Config.ContentVersion = ""

	encoded, err := json.Marshal(bootstrap)
	if err != nil {
		return unknownContentVersion
	}

	sum := sha256.Sum256(encoded)
	return contentVersionPrefix + hex.EncodeToString(sum[:])
}

/*
 * normalizeContentVersion 处理的是“写入成绩时收到的内容版本”。新前端会带 SHA-256 指纹，
 * 也就是 `sha256:` 加 64 个十六进制字符；旧页面、旧脚本或手工调试请求可能没有这个字段。
 * 空值和明显不是服务端指纹形状的值都会统一写成 unknown，表示“这条成绩无法追溯内容包
 * 来源”。这样旧请求仍然能提交，但不会用一个随手写的字符串制造新的排行榜分组。
 */
func normalizeContentVersion(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" || !isKnownContentVersion(normalized) {
		return unknownContentVersion
	}
	return normalized
}

/*
 * optionalContentVersion 用在“按内容版本筛排行榜”这种查询场景。它和 normalizeContentVersion
 * 的区别在于：空字符串在提交成绩时应该写成 unknown，表示这条成绩缺少版本来源；但在读取
 * 排行榜时，空字符串表示旧客户端没有传筛选条件，应该继续看全局排行榜。非法版本筛选也按
 * “没有筛选”处理，避免一个拼错的查询参数把榜单变成空列表。
 */
func optionalContentVersion(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" || !isKnownContentVersion(normalized) {
		return ""
	}
	return normalized
}

func isKnownContentVersion(value string) bool {
	if len(value) > maxContentVersionLen {
		return false
	}
	return value == unknownContentVersion || isGeneratedContentVersion(value)
}

func isGeneratedContentVersion(value string) bool {
	if len(value) != generatedContentVersionLen || !strings.HasPrefix(value, contentVersionPrefix) {
		return false
	}

	for _, character := range value[len(contentVersionPrefix):] {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			continue
		}
		return false
	}

	return true
}
