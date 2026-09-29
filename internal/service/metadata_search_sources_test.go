package service

import (
	"testing"

	"yukihub/internal/appconf"
	enums2 "yukihub/internal/common/enums"
)

func metadataSourceTypes(sources []metadataSearchSource) []enums2.SourceType {
	types := make([]enums2.SourceType, 0, len(sources))
	for _, source := range sources {
		types = append(types, source.source)
	}
	return types
}

func assertMetadataSourceOrder(t *testing.T, got, want []enums2.SourceType) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected source count: got %v want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected source order: got %v want %v", got, want)
		}
	}
}

// 优先级契约：界面配置的顺序即实际尝试顺序，构建器不得重排。
func TestBuildConfiguredMetadataSearchSourcesPreservesConfiguredOrder(t *testing.T) {
	config := &appconf.AppConfig{MetadataSources: []string{"ymgal", "vndb", "steam"}}
	sources := buildConfiguredMetadataSearchSources(metadataSourceDeps{config: config})

	assertMetadataSourceOrder(t, metadataSourceTypes(sources), []enums2.SourceType{
		enums2.Ymgal, enums2.VNDB, enums2.Steam,
	})
}

// 依赖常驻服务对象的来源未注入时应跳过，其余来源保持配置顺序。
func TestBuildConfiguredMetadataSearchSourcesSkipsUnavailableServices(t *testing.T) {
	config := &appconf.AppConfig{MetadataSources: []string{"bangumi", "vndb", "hikarinagi", "ymgal"}}
	sources := buildConfiguredMetadataSearchSources(metadataSourceDeps{config: config})

	assertMetadataSourceOrder(t, metadataSourceTypes(sources), []enums2.SourceType{
		enums2.VNDB, enums2.Ymgal,
	})
}

// 单一入口契约：ImportService 的来源清单必须与共享构建器逐项一致。
func TestImportServiceMetadataSearchSourcesUsesSharedBuilder(t *testing.T) {
	config := &appconf.AppConfig{MetadataSources: []string{"vndb", "steam", "touchgal"}}
	importService := &ImportService{config: config}

	want := metadataSourceTypes(buildConfiguredMetadataSearchSources(metadataSourceDeps{config: config}))
	got := metadataSourceTypes(importService.metadataSearchSources())
	assertMetadataSourceOrder(t, got, want)
}
