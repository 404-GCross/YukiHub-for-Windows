package service

import (
	"context"

	"yukihub/internal/appconf"
	enums2 "yukihub/internal/common/enums"
	"yukihub/internal/service/gamehelper"
	"yukihub/internal/utils/metadata"
)

// metadataSearchSource 描述一个可用的元数据来源及其取数入口。
// 名称搜索与候选搜索统一走 fetchCandidates：数据源未实现候选接口时退化为单结果。
type metadataSearchSource struct {
	source                enums2.SourceType
	fetchByName           func(string) (metadata.MetadataResult, error)
	fetchCandidatesByName func(string) ([]metadata.MetadataResult, error)
}

func (s metadataSearchSource) fetchCandidates(name string) ([]metadata.MetadataResult, error) {
	if s.fetchCandidatesByName != nil {
		return s.fetchCandidatesByName(name)
	}
	result, err := s.fetchByName(name)
	if err != nil {
		return nil, err
	}
	return []metadata.MetadataResult{result}, nil
}

// metadataSourceDeps 是装配元数据来源清单所需的依赖。
// GameService 与 ImportService 的字段同构，故共用同一套装配逻辑。
type metadataSourceDeps struct {
	ctx               context.Context
	config            *appconf.AppConfig
	bangumiService    *BangumiService
	hikarinagiService *HikarinagiService
	nextMoeService    *NextMoeService
}

// buildConfiguredMetadataSearchSources 是元数据来源的唯一入口。
//
// 优先级：直接采用 gamehelper.ConfiguredMetadataSources 返回的用户配置顺序，
// 不做任何重排，因此界面上勾选的顺序即实际尝试顺序。
// 缓存策略：全部来源共用 gamehelper.MetadataGetterOptions 生成的 getter 选项
// （代理、tag 上限、封面来源等），不再有第二份配置。
//
// Bangumi / Hikarinagi 依赖常驻服务对象，未注入时跳过；其余来源按需现场构造 getter。
func buildConfiguredMetadataSearchSources(deps metadataSourceDeps) []metadataSearchSource {
	getterOptions := gamehelper.MetadataGetterOptions(deps.config)
	vndbToken := ""
	language := ""
	if deps.config != nil {
		vndbToken = deps.config.VNDBAccessToken
		language = deps.config.Language
	}

	configuredSources := gamehelper.ConfiguredMetadataSources(deps.config)
	sources := make([]metadataSearchSource, 0, len(configuredSources))
	for _, source := range configuredSources {
		switch source {
		case enums2.Bangumi:
			if deps.bangumiService == nil {
				continue
			}
			sources = append(sources, metadataSearchSource{
				source: enums2.Bangumi,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return deps.bangumiService.fetchMetadataByName(deps.ctx, name)
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return deps.bangumiService.fetchMetadataCandidatesByName(deps.ctx, name)
				},
			})
		case enums2.VNDB:
			getter := metadata.NewVNDBInfoGetterWithLanguage(language, getterOptions...)
			sources = append(sources, metadataSearchSource{
				source: enums2.VNDB,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return getter.FetchMetadataByName(name, vndbToken)
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return metadata.FetchMetadataCandidatesByName(getter, name, vndbToken)
				},
			})
		case enums2.Ymgal:
			getter := metadata.NewYmgalInfoGetter(getterOptions...)
			sources = append(sources, metadataSearchSource{
				source: enums2.Ymgal,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return getter.FetchMetadataByName(name, "")
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return metadata.FetchMetadataCandidatesByName(getter, name, "")
				},
			})
		case enums2.Steam:
			getter := metadata.NewSteamInfoGetterWithLanguage(language, getterOptions...)
			sources = append(sources, metadataSearchSource{
				source: enums2.Steam,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return getter.FetchMetadataByName(name, "")
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return metadata.FetchMetadataCandidatesByName(getter, name, "")
				},
			})
		case enums2.DLsite:
			getter := metadata.NewDLsiteInfoGetter(getterOptions...)
			sources = append(sources, metadataSearchSource{
				source: enums2.DLsite,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return getter.FetchMetadataByName(name, "")
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return metadata.FetchMetadataCandidatesByName(getter, name, "")
				},
			})
		case enums2.ErogameScape:
			getter := metadata.NewErogameScapeInfoGetter(getterOptions...)
			sources = append(sources, metadataSearchSource{
				source: enums2.ErogameScape,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return getter.FetchMetadataByName(name, "")
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return metadata.FetchMetadataCandidatesByName(getter, name, "")
				},
			})
		case enums2.TouchGal:
			getter := metadata.NewTouchGalInfoGetter(getterOptions...)
			sources = append(sources, metadataSearchSource{
				source: enums2.TouchGal,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return getter.FetchMetadataByName(name, "")
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return metadata.FetchMetadataCandidatesByName(getter, name, "")
				},
			})
		case enums2.Hikarinagi:
			if deps.hikarinagiService == nil {
				continue
			}
			sources = append(sources, metadataSearchSource{
				source: enums2.Hikarinagi,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return deps.hikarinagiService.fetchMetadataByName(deps.ctx, name)
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return deps.hikarinagiService.fetchMetadataCandidatesByName(deps.ctx, name)
				},
			})
		case enums2.BangumiMirror:
			// 镜像站复用 Bangumi 服务的令牌与刷新逻辑，仅换 API 基址。
			if deps.bangumiService == nil {
				continue
			}
			sources = append(sources, metadataSearchSource{
				source: enums2.BangumiMirror,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return deps.bangumiService.fetchMirrorMetadataByName(deps.ctx, name)
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return deps.bangumiService.fetchMirrorMetadataCandidatesByName(deps.ctx, name)
				},
			})
		case enums2.NextMoe:
			if deps.nextMoeService == nil {
				continue
			}
			sources = append(sources, metadataSearchSource{
				source: enums2.NextMoe,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return deps.nextMoeService.fetchMetadataByName(deps.ctx, name)
				},
				fetchCandidatesByName: func(name string) ([]metadata.MetadataResult, error) {
					return deps.nextMoeService.fetchMetadataCandidatesByName(deps.ctx, name)
				},
			})
		}
	}
	return sources
}
