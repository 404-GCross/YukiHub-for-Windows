package importer

import (
	"testing"

	"yukihub/internal/common/enums"
	"yukihub/internal/models/yukihub"
)

// 手机版支持的全部来源都必须能映射出身份，映射不出来就会变成 local，
// 进而让 pickYukiHubMetadata 的「偏好来源」判定整体失效。
func TestMapYukiHubSourceTypeCoversEveryMobileSource(t *testing.T) {
	cases := map[string]enums.SourceType{
		"vndb":           enums.VNDB,
		"VNDB":           enums.VNDB,
		"bangumi":        enums.Bangumi,
		"bangumi_mirror": enums.Bangumi,
		"ymgal":          enums.Ymgal,
		"hikarinagi":     enums.Hikarinagi,
		// 回归点：nextmoe 曾经漏在这个 switch 里，导致手机端
		// settings.metadata_source = nextmoe 时整条偏好链路失效
		"nextmoe": enums.NextMoe,
		"steam":   enums.Steam,
		// 完全未知的来源仍按 local 处理
		"":        enums.Local,
		"unknown": enums.Local,
	}

	for raw, want := range cases {
		if got := mapYukiHubSourceType(raw); got != want {
			t.Errorf("mapYukiHubSourceType(%q) = %q, want %q", raw, got, want)
		}
	}
}

// 兜底优先级名单必须涵盖手机端支持的全部来源，漏掉的那个来源的游戏会被
// 错误地归到名单里靠前的来源（用户报的「nextmoe 变成 vndb」）。
func TestYukiHubFallbackSourceOrderCoversEveryMobileSource(t *testing.T) {
	seen := make(map[enums.SourceType]struct{}, len(yukiHubFallbackSourceOrder))
	for _, sourceType := range yukiHubFallbackSourceOrder {
		if _, exists := seen[sourceType]; exists {
			t.Errorf("兜底优先级里 %q 重复", sourceType)
		}
		seen[sourceType] = struct{}{}
	}

	for _, sourceType := range []enums.SourceType{
		enums.VNDB,
		enums.Bangumi,
		enums.Ymgal,
		enums.Hikarinagi,
		enums.NextMoe,
	} {
		if _, exists := seen[sourceType]; !exists {
			t.Errorf("兜底优先级缺少 %q", sourceType)
		}
	}
}

func TestPickYukiHubMetadataHonoursNextMoePreference(t *testing.T) {
	items := []parsedYukiHubMetadata{
		{source: enums.VNDB, sourceID: "v40520", data: yukihub.Metadata{ChineseTitle: "vndb 侧标题"}},
		{source: enums.NextMoe, sourceID: "12345", data: yukihub.Metadata{ChineseTitle: "nextmoe 侧标题"}},
	}

	primary := pickYukiHubMetadata(items, "nextmoe")
	if primary.source != enums.NextMoe {
		t.Fatalf("primary.source = %q, want nextmoe", primary.source)
	}
	if primary.sourceID != "12345" {
		t.Errorf("primary.sourceID = %q, want 12345", primary.sourceID)
	}
	if primary.data.ChineseTitle != "nextmoe 侧标题" {
		t.Errorf("没有采用 nextmoe 侧的数据: %q", primary.data.ChineseTitle)
	}

	// 备份里的偏好来源在没有对应条目时，仍应落到兜底名单
	fallback := pickYukiHubMetadata(items, "ymgal")
	if fallback.source != enums.VNDB {
		t.Errorf("兜底结果 = %q, want vndb", fallback.source)
	}
}

// 只有 nextmoe 条目的游戏（偏好来源为空时）也不能退化成 local。
func TestPickYukiHubMetadataKeepsNextMoeWhenItIsTheOnlySource(t *testing.T) {
	items := []parsedYukiHubMetadata{
		{source: enums.NextMoe, sourceID: "777", data: yukihub.Metadata{ChineseTitle: "未萌目录条目"}},
	}

	primary := pickYukiHubMetadata(items, "")
	if primary.source != enums.NextMoe {
		t.Errorf("primary.source = %q, want nextmoe", primary.source)
	}
	if primary.sourceID != "777" {
		t.Errorf("primary.sourceID = %q, want 777", primary.sourceID)
	}
}
