package yukihubaccount

import (
	"context"
	"net/http"
	"testing"
)

// 资料页「正在玩」的结构化字段。
//
// 服务端目前只下发 activity 这一句拼好的中文文案（"正在玩：xxx"），没有开始
// 时间，客户端算不出「已玩多久」。这里先把别名解析钉住：服务端将来补上
// playingGame / playingStartedAt 之后，资料卡立刻能显示实时时长，不需要再改
// 客户端 —— 但只有解析确实生效时才算数，所以用测试锁住。

func TestUserProfileParsesStructuredPlayingFields(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		game  string
		start int64
	}{
		{
			name:  "驼峰",
			body:  `{"uid":7,"nickname":"Yuki","activity":"正在玩：樱之刻","playingGame":"樱之刻","playingStartedAt":1735689600000}`,
			game:  "樱之刻",
			start: 1735689600000,
		},
		{
			name:  "下划线",
			body:  `{"uid":7,"nickname":"Yuki","activity":"正在玩：樱之刻","playing_game":"樱之刻","playing_started_at":1735689600001}`,
			game:  "樱之刻",
			start: 1735689600001,
		},
		{
			name:  "同义命名 playStartedAt",
			body:  `{"uid":7,"nickname":"Yuki","activity":"正在玩：CLANNAD","playStartedAt":1735689600002}`,
			game:  "",
			start: 1735689600002,
		},
		{
			name:  "同义命名 startedAt",
			body:  `{"uid":7,"nickname":"Yuki","activity":"正在玩：CLANNAD","startedAt":1735689600003}`,
			game:  "",
			start: 1735689600003,
		},
		{
			// 服务端还没上这些字段时不能报错，也不能凭空造出时间
			name:  "只有旧字段",
			body:  `{"uid":7,"nickname":"Yuki","activity":"正在玩：CLANNAD"}`,
			game:  "",
			start: 0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server, _, _ := newRecordingServer(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, testCase.body)
			})

			client := NewClient(server.URL)
			profile, err := client.UserProfile(context.Background(), "token", 7)
			if err != nil {
				t.Fatalf("UserProfile: %v", err)
			}
			if profile.PlayingGame != testCase.game {
				t.Errorf("PlayingGame = %q，期望 %q", profile.PlayingGame, testCase.game)
			}
			if profile.PlayingStartedAt != testCase.start {
				t.Errorf("PlayingStartedAt = %d，期望 %d", profile.PlayingStartedAt, testCase.start)
			}
			if profile.Activity != "正在玩：樱之刻" && profile.Activity != "正在玩：CLANNAD" {
				t.Errorf("Activity 被改动了：%q", profile.Activity)
			}
		})
	}
}
