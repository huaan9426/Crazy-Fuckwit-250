package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testContentVersion = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validRunForTest(username string) runSubmission {
	return runSubmission{
		Username:       username,
		DurationMs:     120_000,
		MaxSingleSpend: 42_000,
		FinalBalance:   0,
		TotalSpent:     2_500_000,
		TotalIncome:    0,
		EndedBy:        "balance_zero",
		ChaosSeed:      "test-seed",
		ContentVersion: testContentVersion,
	}
}

func validBootstrapForTest() bootstrapResponse {
	return bootstrapResponse{
		Config: gameConfig{
			InitialBalance: defaultInitialBalance,
			RoundLimitMs:   defaultRoundLimitMs,
			DefaultMode:    defaultMode,
			BalanceTuning:  deriveBalanceTuning(defaultInitialBalance, defaultRoundLimitMs),
		},
		Items:       testItemsForBootstrap(),
		Scenes:      testScenesForBootstrap(),
		Events:      testEventsForBootstrap(),
		Endings:     testEndingsForBootstrap(),
		Statuses:    testStatusesForBootstrap(),
		AudioTracks: testAudioTracksForBootstrap(),
	}
}

func TestContentVersionIsStableAndChangesWithContent(t *testing.T) {
	first := withContentVersion(validBootstrapForTest())
	second := withContentVersion(validBootstrapForTest())

	/*
	 * 内容版本会被前端原样带回成绩提交，所以它必须同时满足两个条件：同一份内容包重复
	 * 计算时不能漂移，否则同一批成绩会被误认为来自不同版本；内容包里真正影响玩法的
	 * 金额发生变化时也必须改变，否则后续调参后旧成绩和新成绩会混在一起。
	 */
	if first.Config.ContentVersion == "" || first.Config.ContentVersion == unknownContentVersion {
		t.Fatalf("content version = %q, want generated fingerprint", first.Config.ContentVersion)
	}
	if first.Config.ContentVersion != second.Config.ContentVersion {
		t.Fatalf("content version drifted: first=%q second=%q", first.Config.ContentVersion, second.Config.ContentVersion)
	}

	second.Items[0].Price += 1
	second = withContentVersion(second)
	if first.Config.ContentVersion == second.Config.ContentVersion {
		t.Fatalf("content version did not change after gameplay content changed: %q", first.Config.ContentVersion)
	}
}

func TestContentVersionNormalizationKeepsOnlyGeneratedFingerprints(t *testing.T) {
	generated := withContentVersion(validBootstrapForTest()).Config.ContentVersion

	if !isGeneratedContentVersion(generated) {
		t.Fatalf("generated content version %q should match the server fingerprint shape", generated)
	}

	tests := []struct {
		name       string
		value      string
		normalized string
		optional   string
	}{
		{name: "empty", value: "", normalized: unknownContentVersion, optional: ""},
		{name: "blank", value: "   ", normalized: unknownContentVersion, optional: ""},
		{name: "generated", value: " " + generated + " ", normalized: generated, optional: generated},
		{name: "unknown", value: unknownContentVersion, normalized: unknownContentVersion, optional: unknownContentVersion},
		{name: "manual version", value: "test-content-version", normalized: unknownContentVersion, optional: ""},
		{name: "bad hash", value: "sha256:not-a-real-hash", normalized: unknownContentVersion, optional: ""},
		{name: "uppercase hash", value: "SHA256:0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF", normalized: unknownContentVersion, optional: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeContentVersion(test.value); got != test.normalized {
				t.Fatalf("normalizeContentVersion(%q) = %q, want %q", test.value, got, test.normalized)
			}
			if got := optionalContentVersion(test.value); got != test.optional {
				t.Fatalf("optionalContentVersion(%q) = %q, want %q", test.value, got, test.optional)
			}
		})
	}
}

func TestMemoryFallbackBootstrapCoversRepresentativeLocalMechanics(t *testing.T) {
	bootstrap := withContentVersion(bootstrapContent())
	if !isGeneratedContentVersion(bootstrap.Config.ContentVersion) {
		t.Fatalf("memory fallback content version = %q, want generated fingerprint", bootstrap.Config.ContentVersion)
	}

	/*
	 * bootstrapContent 是没有 PostgreSQL 时的 Go 内存兜底，它不应该伪装成完整内容库，
	 * 也不需要达到数据库的 300 张卡规模。但它必须能覆盖几个本地调试时最容易误判的
	 * 机制：coin/small 低价卡让十倍、二十倍倍率规则有候选；income 卡让反向进账逻辑
	 * 能被看到；shock 卡让高额消费、VISA 选择最高消费和特殊场景仍有代表样例。
	 */
	requiredTiers := []string{"coin", "small", "daily", "income", "shock"}
	seenTiers := make(map[string]struct{})
	lowestSpendItemPrice := int64(0)
	for _, item := range bootstrap.Items {
		seenTiers[item.Tier] = struct{}{}
		if item.Tier != "income" && item.Price > 0 && (lowestSpendItemPrice == 0 || item.Price < lowestSpendItemPrice) {
			lowestSpendItemPrice = item.Price
		}
	}
	for _, tier := range requiredTiers {
		if _, exists := seenTiers[tier]; !exists {
			t.Fatalf("memory fallback bootstrap is missing representative item tier %q", tier)
		}
	}
	if lowestSpendItemPrice > 1 {
		t.Fatalf("memory fallback lowest spend item price = %d, want a 1-yuan spend card", lowestSpendItemPrice)
	}

	/*
	 * 状态效果在前端不是普通文案，它会直接改变刷新速度、大额概率和事件概率。这里不要求
	 * 内存兜底包含数据库里的全部 12 个状态，但至少保留两个压力状态、一个低落状态和一个
	 * 返钱状态，让无数据库开发时仍能看到状态算法的几个关键方向。
	 */
	requiredStatuses := []string{"生气", "低落", "上头", "好运"}
	seenStatuses := make(map[string]struct{})
	for _, effect := range bootstrap.Statuses {
		seenStatuses[effect.Name] = struct{}{}
	}
	for _, name := range requiredStatuses {
		if _, exists := seenStatuses[name]; !exists {
			t.Fatalf("memory fallback bootstrap is missing representative status %q", name)
		}
	}
}

func testScenesForBootstrap() []scene {
	scenes := make([]scene, 0, minimumBootstrapScenes)
	for index := 0; index < minimumBootstrapScenes; index += 1 {
		rarity := "rare"
		if index < minimumBootstrapCommonScenes {
			rarity = "common"
		} else if index%5 == 0 {
			rarity = "wild"
		}

		scenes = append(scenes, scene{
			ID:          fmt.Sprintf("test-scene-%02d", index),
			Name:        fmt.Sprintf("测试场景%02d", index),
			DurationSec: 35,
			Rarity:      rarity,
			RiskLevel:   int64(index%4 + 1),
			ItemTags:    []string{"test", fmt.Sprintf("scene-tag-%02d", index)},
			EventTags:   []string{"income", "fee"},
			Modes:       []string{defaultMode},
		})
	}

	return scenes
}

func testItemsForBootstrap() []item {
	categories := []string{
		"日常小额", "交通", "社交压力", "平台规则", "灾难维修", "数码意外", "亲子教育", "教育考试",
		"职场晋升", "健康", "宠物", "法律行政", "旅行", "大件现实", "婚礼", "富人体验",
		"高端误操作", "车主成本", "反向进账", "赔付",
	}
	tiers := []struct {
		name  string
		price int64
	}{
		{name: "coin", price: 1},
		{name: "small", price: 220},
		{name: "daily", price: 520},
		{name: "premium", price: 1_800},
		{name: "large", price: 12_000},
		{name: "heavy", price: 88_000},
		{name: "shock", price: 320_000},
		{name: "income", price: 5_000},
	}

	items := []item{{
		ID:       "test-item",
		Name:     "测试商品",
		Category: "测试",
		Price:    100,
		Tier:     "daily",
		Weight:   1,
		Modes:    []string{defaultMode},
		Tags:     []string{"test"},
	}, {
		ID:       "test-income",
		Name:     "测试入账",
		Category: "测试",
		Price:    50,
		Tier:     "income",
		Weight:   1,
		Modes:    []string{defaultMode},
		Tags:     []string{"income"},
	}}

	for index := 2; len(items) < minimumBootstrapItems; index += 1 {
		tier := tiers[index%len(tiers)]
		category := categories[index%len(categories)]
		sceneID := fmt.Sprintf("test-scene-%02d", index%minimumBootstrapItemScenes)
		items = append(items, item{
			ID:         fmt.Sprintf("test-item-%03d", index),
			Name:       fmt.Sprintf("测试商品%03d", index),
			Category:   category,
			SceneID:    ptrString(sceneID),
			Price:      tier.price + int64(index%5)*10,
			Tier:       tier.name,
			Weight:     1,
			MinBalance: 0,
			Modes:      []string{defaultMode},
			Tags:       []string{"test", fmt.Sprintf("category-%02d", index%len(categories))},
		})
	}

	return items
}

func testEventsForBootstrap() []gameEvent {
	events := []gameEvent{{
		ID:          "test-spend-event",
		Title:       "测试扣款事件",
		Description: "测试扣款描述",
		Delta:       ptrDelta(-20),
		Probability: 0.1,
		Modes:       []string{defaultMode},
		Tags:        []string{"test"},
	}, {
		ID:          "test-income-event",
		Title:       "测试返钱事件",
		Description: "测试返钱描述",
		Delta:       ptrDelta(10),
		Probability: 0.1,
		Modes:       []string{defaultMode},
		Tags:        []string{"income"},
	}}

	for index := 2; len(events) < minimumBootstrapEvents; index += 1 {
		events = append(events, gameEvent{
			ID:          fmt.Sprintf("test-event-%03d", index),
			Title:       fmt.Sprintf("测试事件%03d", index),
			Description: "测试事件描述",
			Probability: 0.05,
			Modes:       []string{defaultMode},
			Tags:        []string{"test"},
		})
	}

	return events
}

func testEndingsForBootstrap() []terminalEvent {
	endings := []terminalEvent{{
		ID:            "test-zero-ending",
		Title:         "测试清零终局",
		Description:   "测试清零终局描述",
		Probability:   0.0007,
		MinRiskLevel:  1,
		BalanceEffect: "zero",
		Tags:          []string{"test", "ending"},
		Modes:         []string{defaultMode},
	}}

	for index := 1; len(endings) < minimumBootstrapEndings; index += 1 {
		endings = append(endings, terminalEvent{
			ID:            fmt.Sprintf("test-ending-%02d", index),
			Title:         fmt.Sprintf("测试终局%02d", index),
			Description:   "测试终局描述",
			Probability:   0.0003,
			MinRiskLevel:  1,
			BalanceEffect: "none",
			Tags:          []string{"test", "ending"},
			Modes:         []string{defaultMode},
		})
	}

	return endings
}

func testStatusesForBootstrap() []statusEffect {
	statuses := []statusEffect{{
		ID:                    "test-status",
		Name:                  "测试状态",
		DurationSec:           10,
		ItemRefreshMultiplier: 0.9,
		HighPriceMultiplier:   1.2,
		EventMultiplier:       1.1,
		Tags:                  []string{"test"},
		Description:           "测试状态描述",
	}}

	for index := 1; len(statuses) < minimumBootstrapStatuses; index += 1 {
		statuses = append(statuses, statusEffect{
			ID:                    fmt.Sprintf("test-status-%02d", index),
			Name:                  fmt.Sprintf("测试状态%02d", index),
			DurationSec:           10,
			ItemRefreshMultiplier: 1,
			HighPriceMultiplier:   1,
			EventMultiplier:       1,
			Tags:                  []string{"test"},
			Description:           "测试状态描述",
		})
	}

	return statuses
}

func testAudioTracksForBootstrap() []audioTrack {
	tracks := make([]audioTrack, 0, minimumBootstrapAudioTracks)
	moods := []string{"menu", "rush", "settlement"}
	for index := 0; index < minimumBootstrapAudioTracks; index += 1 {
		tracks = append(tracks, audioTrack{
			ID:      fmt.Sprintf("test-track-%02d", index),
			Title:   fmt.Sprintf("测试音轨%02d", index),
			Mood:    moods[index%len(moods)],
			License: "custom",
		})
	}

	return tracks
}

func TestUsernameNormalizationAndValidation(t *testing.T) {
	normalized := normalizeUsername("  今晚   就花完  ")
	if normalized != "今晚 就花完" {
		t.Fatalf("normalizeUsername collapsed whitespace incorrectly: %q", normalized)
	}

	if !validUsername(normalized) {
		t.Fatalf("expected normalized username to be valid")
	}

	for _, username := range []string{
		"短",
		"带/斜杠",
		"带<尖括号>",
		"前后空格 ",
		"多个  空格",
		"带\t制表",
		"这个用户名长度已经明显超过十六个字符限制",
	} {
		if validUsername(username) {
			t.Fatalf("expected username %q to be rejected", username)
		}
	}
}

func TestReserveUsernameMemoryFallbackRenewsMatchingReservationToken(t *testing.T) {
	state := &appState{usernames: map[string]usernameReservation{}}
	now := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)

	reserved, token, err := state.reserveUsernameInMemory("刷新续租", "", now)
	if err != nil {
		t.Fatalf("reserve username: %v", err)
	}
	if !reserved || token == "" {
		t.Fatalf("first reserve = (%v, %q), want reserved with token", reserved, token)
	}

	reserved, _, err = state.reserveUsernameInMemory("刷新续租", "", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second reserve without token: %v", err)
	}
	if reserved {
		t.Fatalf("reserve without matching token should stay blocked while lease is active")
	}

	reserved, renewedToken, err := state.reserveUsernameInMemory("刷新续租", token, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("renew reserve with token: %v", err)
	}
	if !reserved || renewedToken != token {
		t.Fatalf("renewed reserve = (%v, %q), want original token %q", reserved, renewedToken, token)
	}
}

func TestReserveUsernameMemoryFallbackReleasesExpiredReservation(t *testing.T) {
	now := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)
	state := &appState{
		usernames: map[string]usernameReservation{
			"过期预约": {
				Token:         "old-token",
				ReservedUntil: now.Add(-time.Minute),
			},
		},
	}

	reserved, token, err := state.reserveUsernameInMemory("过期预约", "", now)
	if err != nil {
		t.Fatalf("reserve expired username: %v", err)
	}
	if !reserved || token == "" || token == "old-token" {
		t.Fatalf("reserve expired username = (%v, %q), want a fresh token", reserved, token)
	}
}

func TestSubmitRunMemoryFallbackRequiresMatchingReservationToken(t *testing.T) {
	state := &appState{usernames: map[string]usernameReservation{}}
	/*
	 * 这个测试会通过 handleSubmitRun 走完整 HTTP 处理链，而 handleSubmitRun 内部用
	 * time.Now() 判断用户名预约是否仍然有效。这里不能使用固定历史时间，否则当真实时钟
	 * 走过那段 30 分钟租约后，测试会把“仍在预约期内”的场景误测成“预约已经过期”，从而
	 * 让无 token 的提交被错误放行。
	 */
	now := time.Now()
	reserved, token, err := state.reserveUsernameInMemory("预约提交", "", now)
	if err != nil {
		t.Fatalf("reserve username: %v", err)
	}
	if !reserved || token == "" {
		t.Fatalf("reserve = (%v, %q), want token", reserved, token)
	}

	body, err := json.Marshal(validRunForTest("预约提交"))
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(string(body)))
	blockedResponse := httptest.NewRecorder()
	state.handleSubmitRun(blockedResponse, blockedRequest)
	if blockedResponse.Code != http.StatusConflict {
		t.Fatalf("submit without token status = %d, want %d; body: %s", blockedResponse.Code, http.StatusConflict, blockedResponse.Body.String())
	}

	run := validRunForTest("预约提交")
	run.ReservationToken = token
	authorizedBody, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal authorized run: %v", err)
	}
	authorizedRequest := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(string(authorizedBody)))
	authorizedResponse := httptest.NewRecorder()
	state.handleSubmitRun(authorizedResponse, authorizedRequest)
	if authorizedResponse.Code != http.StatusCreated {
		t.Fatalf("submit with token status = %d, want %d; body: %s", authorizedResponse.Code, http.StatusCreated, authorizedResponse.Body.String())
	}
}

func TestValidateBootstrapContentRequiresCoreCategories(t *testing.T) {
	if err := validateBootstrapContent(validBootstrapForTest()); err != nil {
		t.Fatalf("expected complete bootstrap to be accepted: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{name: "items", mutate: func(bootstrap *bootstrapResponse) { bootstrap.Items = nil }},
		{name: "scenes", mutate: func(bootstrap *bootstrapResponse) { bootstrap.Scenes = nil }},
		{name: "events", mutate: func(bootstrap *bootstrapResponse) { bootstrap.Events = nil }},
		{name: "endings", mutate: func(bootstrap *bootstrapResponse) { bootstrap.Endings = nil }},
		{name: "statuses", mutate: func(bootstrap *bootstrapResponse) { bootstrap.Statuses = nil }},
		{name: "audio tracks", mutate: func(bootstrap *bootstrapResponse) { bootstrap.AudioTracks = nil }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected missing %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresDefaultModeArrays(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "scene mode missing default",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Scenes[0].Modes = []string{"other-mode"}
			},
		},
		{
			name: "item mode missing default",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].Modes = nil
			},
		},
		{
			name: "event mode missing default",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events[0].Modes = []string{}
			},
		},
		{
			name: "terminal event mode missing default",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].Modes = []string{"other-mode"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRejectsSceneDurationDrift(t *testing.T) {
	bootstrap := validBootstrapForTest()
	bootstrap.Scenes[0].DurationSec = 18

	if err := validateBootstrapContent(bootstrap); err == nil {
		t.Fatalf("expected scene duration drift to be rejected")
	}
}

func TestValidateBootstrapContentRejectsNegativeSceneMoneyGates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "negative entry cost",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Scenes[0].EntryCost = -1
			},
		},
		{
			name: "negative min balance",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Scenes[0].MinBalance = -1
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresSceneMix(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "too few common scenes",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := 0; index < minimumBootstrapCommonScenes; index += 1 {
					bootstrap.Scenes[index].Rarity = "rare"
				}
			},
		},
		{
			name: "too few special scenes",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Scenes {
					bootstrap.Scenes[index].Rarity = "common"
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresPlanScaleAndPriceCoverage(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "too few items",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items = bootstrap.Items[:minimumBootstrapItems-1]
			},
		},
		{
			name: "too few scenes",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Scenes = bootstrap.Scenes[:minimumBootstrapScenes-1]
			},
		},
		{
			name: "too few events",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events = bootstrap.Events[:minimumBootstrapEvents-1]
			},
		},
		{
			name: "too few statuses",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses = bootstrap.Statuses[:minimumBootstrapStatuses-1]
			},
		},
		{
			name: "too few endings",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings = bootstrap.Endings[:minimumBootstrapEndings-1]
			},
		},
		{
			name: "too few audio tracks",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.AudioTracks = bootstrap.AudioTracks[:minimumBootstrapAudioTracks-1]
			},
		},
		{
			name: "missing small tier",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Tier == "small" {
						bootstrap.Items[index].Tier = "daily"
					}
				}
			},
		},
		{
			name: "undercovered small tier",
			mutate: func(bootstrap *bootstrapResponse) {
				keptSmallTier := false
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Tier != "small" {
						continue
					}
					if !keptSmallTier {
						keptSmallTier = true
						continue
					}
					bootstrap.Items[index].Tier = "daily"
				}
			},
		},
		{
			name: "too few item categories",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					bootstrap.Items[index].Category = "单一分类"
				}
			},
		},
		{
			name: "too few linked item scenes",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					bootstrap.Items[index].SceneID = nil
				}
			},
		},
		{
			name: "item references missing scene",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[2].SceneID = ptrString("missing-scene")
			},
		},
		{
			name: "missing change level price",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Price <= 100 {
						bootstrap.Items[index].Price = 180
					}
				}
			},
		},
		{
			name: "change level price only income",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Tier == "income" {
						bootstrap.Items[index].Price = 50
						continue
					}
					if bootstrap.Items[index].Price <= 100 {
						bootstrap.Items[index].Price = 180
					}
				}
			},
		},
		{
			name: "too few change level prices",
			mutate: func(bootstrap *bootstrapResponse) {
				keptChangePrices := 0
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Tier == "income" || bootstrap.Items[index].Price > 50 {
						continue
					}
					keptChangePrices += 1
					if keptChangePrices >= minimumChangeSpendItems {
						bootstrap.Items[index].Price = 68
					}
				}
			},
		},
		{
			name: "missing shock price",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Price >= 300_000 {
						bootstrap.Items[index].Price = 250_000
					}
				}
			},
		},
		{
			name: "shock price only income",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Tier == "income" {
						bootstrap.Items[index].Price = 320_000
						continue
					}
					if bootstrap.Items[index].Price >= 300_000 {
						bootstrap.Items[index].Price = 250_000
					}
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRejectsUnsupportedAudioContract(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "unsupported audio mood",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.AudioTracks[0].Mood = "panic"
			},
		},
		{
			name: "unsupported audio license",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.AudioTracks[0].License = "unknown"
			},
		},
		{
			name: "blank audio title",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.AudioTracks[0].Title = " "
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresUsableBalanceTuning(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "target clear reaches hard round limit",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.TargetClearMs = bootstrap.Config.RoundLimitMs
			},
		},
		{
			name: "stages do not cover target clear",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.StageDurationMs = 1
			},
		},
		{
			name: "hand refresh not longer than checkout lock",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.HandRefreshMs = bootstrap.Config.BalanceTuning.SelectionSettleMs
			},
		},
		{
			name: "first interest after hard round limit",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.InterestStartDelayMs = bootstrap.Config.RoundLimitMs
			},
		},
		{
			name: "missing zero interest band",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.InterestBands = []interestBand{{MinBalance: 100, Rate: 0.02}}
			},
		},
		{
			name: "low balance interest rate lower than high balance",
			mutate: func(bootstrap *bootstrapResponse) {
				bands := bootstrap.Config.BalanceTuning.InterestBands
				bands[len(bands)-1].Rate = 0.001
				bootstrap.Config.BalanceTuning.InterestBands = bands
			},
		},
		{
			name: "clear cart picks more than one hand",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.ClearCartPickCount = visibleCardCount + 1
			},
		},
		{
			name: "normal high card chance above contract",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.NormalHighCardHandChance = 0.11
			},
		},
		{
			name: "missing x1 multiplier fallback",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Config.BalanceTuning.MultiplierRules = []multiplierRule{{
					ID:            "x2",
					Label:         "x2",
					Multiplier:    2,
					MinBalance:    0,
					MaxUnitPrice:  100,
					MaxTotalPrice: 200,
					Weight:        1,
				}}
			},
		},
		{
			name: "higher multiplier has wider unit limit",
			mutate: func(bootstrap *bootstrapResponse) {
				rules := bootstrap.Config.BalanceTuning.MultiplierRules
				rules[len(rules)-1].MaxUnitPrice = rules[1].MaxUnitPrice + 1
				bootstrap.Config.BalanceTuning.MultiplierRules = rules
			},
		},
		{
			name: "higher multiplier has wider total limit",
			mutate: func(bootstrap *bootstrapResponse) {
				rules := bootstrap.Config.BalanceTuning.MultiplierRules
				rules[len(rules)-1].MaxTotalPrice = rules[1].MaxTotalPrice + 1
				bootstrap.Config.BalanceTuning.MultiplierRules = rules
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresCommonScene(t *testing.T) {
	bootstrap := validBootstrapForTest()
	bootstrap.Scenes[0].Rarity = "wild"

	if err := validateBootstrapContent(bootstrap); err == nil {
		t.Fatalf("expected bootstrap without a common scene to be rejected")
	}
}

func TestValidateBootstrapContentRequiresPlayableItems(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "missing spend item",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items = []item{bootstrap.Items[1]}
			},
		},
		{
			name: "missing income item",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items = []item{bootstrap.Items[0]}
			},
		},
		{
			name: "spend item not payable at initial balance",
			mutate: func(bootstrap *bootstrapResponse) {
				for index := range bootstrap.Items {
					if bootstrap.Items[index].Tier != "income" {
						bootstrap.Items[index].Price = bootstrap.Config.InitialBalance + 1
					}
				}
			},
		},
		{
			name: "zero priced item",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].Price = 0
			},
		},
		{
			name: "negative priced item",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].Price = -1
			},
		},
		{
			name: "non-positive max buy",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].MaxBuy = ptrInt64(0)
			},
		},
		{
			name: "negative max buy",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].MaxBuy = ptrInt64(-1)
			},
		},
		{
			name: "zero item weight",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].Weight = 0
			},
		},
		{
			name: "negative item weight",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].Weight = -1
			},
		},
		{
			name: "negative item min balance",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Items[0].MinBalance = -1
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresSpendAndIncomeEvents(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "missing spend event",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events = []gameEvent{bootstrap.Events[1]}
			},
		},
		{
			name: "missing income event",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events = []gameEvent{bootstrap.Events[0]}
			},
		},
		{
			name: "only neutral event",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events = []gameEvent{{
					ID:          "test-neutral-event",
					Title:       "测试提示事件",
					Description: "这条事件只写流水，不改变金额。",
					Probability: 0.1,
					Modes:       []string{defaultMode},
					Tags:        []string{"test"},
				}}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRejectsUnsafeEventTuning(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "zero probability",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events[0].Probability = 0
			},
		},
		{
			name: "probability too high",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events[0].Probability = maximumChaosEventProbability + 0.01
			},
		},
		{
			name: "spend delta too large",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events[0].Delta = ptrDelta(-(bootstrap.Config.InitialBalance/maximumChaosEventDeltaDivisor + 1))
			},
		},
		{
			name: "income delta too large",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events[1].Delta = ptrDelta(bootstrap.Config.InitialBalance/maximumChaosEventDeltaDivisor + 1)
			},
		},
		{
			name: "negative cooldown",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Events[0].CooldownSec = -1
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresEventMatchingTags(t *testing.T) {
	bootstrap := validBootstrapForTest()
	bootstrap.Events[0].Tags = []string{"never-matches-anything"}

	if err := validateBootstrapContent(bootstrap); err == nil {
		t.Fatalf("expected event with unmatched tags to be rejected")
	}
}

func TestValidateBootstrapContentRequiresEffectiveStatuses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "missing refresh change",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].ItemRefreshMultiplier = 1
			},
		},
		{
			name: "missing high price change",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].HighPriceMultiplier = 1
			},
		},
		{
			name: "missing event change",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].EventMultiplier = 1
			},
		},
		{
			name: "non-positive multiplier",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].EventMultiplier = 0
			},
		},
		{
			name: "duration too short",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].DurationSec = minimumStatusDurationSec - 1
			},
		},
		{
			name: "duration too long",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].DurationSec = maximumStatusDurationSec + 1
			},
		},
		{
			name: "refresh multiplier too low",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].ItemRefreshMultiplier = 0.49
			},
		},
		{
			name: "refresh multiplier too high",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].ItemRefreshMultiplier = 1.81
			},
		},
		{
			name: "high price multiplier too low",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].HighPriceMultiplier = 0.49
			},
		},
		{
			name: "high price multiplier too high",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].HighPriceMultiplier = 2.01
			},
		},
		{
			name: "event multiplier too high",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].EventMultiplier = 1.81
			},
		},
		{
			name: "event multiplier too low",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].EventMultiplier = 0.49
			},
		},
		{
			name: "missing matching tags",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Statuses[0].Tags = nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateBootstrapContentRequiresTriggerableTerminalEvents(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bootstrapResponse)
	}{
		{
			name: "terminal event after round limit",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].MinElapsedMs = bootstrap.Config.RoundLimitMs
			},
		},
		{
			name: "terminal event probability too high",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].Probability = maxTerminalEventProbability + 0.001
			},
		},
		{
			name: "terminal event zero probability",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].Probability = 0
			},
		},
		{
			name: "terminal event negative elapsed",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].MinElapsedMs = -1
			},
		},
		{
			name: "terminal event zero max balance",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].MaxBalance = ptrInt64(0)
			},
		},
		{
			name: "terminal event risk too low",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].MinRiskLevel = 0
			},
		},
		{
			name: "terminal event unsupported balance effect",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].BalanceEffect = "refund"
			},
		},
		{
			name: "terminal event requires impossible risk",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].MinRiskLevel = 5
			},
		},
		{
			name: "terminal event has unmatched tags",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].Tags = []string{"unmatched-tag", "ending"}
			},
		},
		{
			name: "missing zero terminal event",
			mutate: func(bootstrap *bootstrapResponse) {
				bootstrap.Endings[0].BalanceEffect = "none"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bootstrap := validBootstrapForTest()
			test.mutate(&bootstrap)

			if err := validateBootstrapContent(bootstrap); err == nil {
				t.Fatalf("expected %s to be rejected", test.name)
			}
		})
	}
}

func TestValidateRunRejectsInvalidSubmissions(t *testing.T) {
	tests := []struct {
		name string
		run  runSubmission
	}{
		{
			name: "duration too short",
			run: func() runSubmission {
				run := validRunForTest("时间太短")
				run.DurationMs = minAcceptedDurationMs - 1
				return run
			}(),
		},
		{
			name: "duration after hard round limit",
			run: func() runSubmission {
				run := validRunForTest("时间超限")
				run.DurationMs = defaultRoundLimitMs + 1
				return run
			}(),
		},
		{
			name: "negative money",
			run: func() runSubmission {
				run := validRunForTest("金额非法")
				run.TotalSpent = -1
				return run
			}(),
		},
		{
			name: "money fields too large",
			run: func() runSubmission {
				run := validRunForTest("金额过大")
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				run.FinalBalance = maxAcceptedRunMoney + 1
				run.TotalSpent = 0
				return run
			}(),
		},
		{
			name: "max single spend exceeds total spent",
			run: func() runSubmission {
				run := validRunForTest("单笔超限")
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				run.TotalSpent = 10_000
				run.MaxSingleSpend = 10_001
				run.FinalBalance = defaultInitialBalance - run.TotalSpent
				return run
			}(),
		},
		{
			name: "money fields do not balance",
			run: func() runSubmission {
				run := validRunForTest("账目不平")
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				run.TotalSpent = 1_000
				run.MaxSingleSpend = 500
				run.FinalBalance = 123
				return run
			}(),
		},
		{
			name: "balance zero with remaining balance",
			run: func() runSubmission {
				run := validRunForTest("清空不零")
				run.TotalSpent = defaultInitialBalance - 100
				run.FinalBalance = 100
				return run
			}(),
		},
		{
			name: "timeout with zero balance",
			run: func() runSubmission {
				run := validRunForTest("超时零余额")
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				return run
			}(),
		},
		{
			name: "manual with zero balance",
			run: func() runSubmission {
				run := validRunForTest("手动清零")
				run.EndedBy = "manual"
				return run
			}(),
		},
		{
			name: "timeout before hard round limit",
			run: func() runSubmission {
				run := validRunForTest("提前超时")
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs - 1
				run.FinalBalance = 10_000
				run.TotalSpent = defaultInitialBalance + run.TotalIncome - run.FinalBalance
				return run
			}(),
		},
		{
			name: "unknown end reason",
			run: func() runSubmission {
				run := validRunForTest("原因非法")
				run.EndedBy = "stage_done"
				return run
			}(),
		},
		{
			name: "terminal event needs id",
			run: func() runSubmission {
				run := validRunForTest("终局缺标识")
				run.EndedBy = "terminal_event"
				run.EndingTitle = "特殊终局"
				run.EndingDetail = "终局详情"
				return run
			}(),
		},
		{
			name: "terminal event needs title",
			run: func() runSubmission {
				run := validRunForTest("终局缺名")
				run.EndedBy = "terminal_event"
				run.EndingID = "test-ending"
				run.EndingDetail = "终局详情"
				return run
			}(),
		},
		{
			name: "terminal event needs detail",
			run: func() runSubmission {
				run := validRunForTest("终局缺详情")
				run.EndedBy = "terminal_event"
				run.EndingID = "test-ending"
				run.EndingTitle = "特殊终局"
				return run
			}(),
		},
		{
			name: "non-terminal cannot carry terminal event fields",
			run: func() runSubmission {
				run := validRunForTest("普通结算夹带终局")
				run.EndingID = "test-ending"
				run.EndingTitle = "不该出现的终局"
				run.EndingDetail = "普通清空成绩不应该夹带终局文案"
				return run
			}(),
		},
		{
			name: "missing chaos seed",
			run: func() runSubmission {
				run := validRunForTest("缺种子")
				run.ChaosSeed = ""
				return run
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateRun(test.run); err == nil {
				t.Fatalf("expected validateRun to reject %s", test.name)
			}
		})
	}
}

func TestValidateRunAcceptsTerminalEventWithCompleteFields(t *testing.T) {
	run := validRunForTest("终局有效")
	run.EndedBy = "terminal_event"
	run.EndingID = "test-ending"
	run.EndingTitle = "特殊终局"
	run.EndingDetail = "终局详情"

	if err := validateRun(run); err != nil {
		t.Fatalf("expected terminal event with complete fields to be accepted: %v", err)
	}
}

func TestValidateRunAcceptsTimeoutAtHardRoundLimit(t *testing.T) {
	run := validRunForTest("时间到有效")
	run.EndedBy = "timeout"
	run.DurationMs = defaultRoundLimitMs
	run.FinalBalance = 10_000
	run.TotalSpent = defaultInitialBalance + run.TotalIncome - run.FinalBalance

	if err := validateRun(run); err != nil {
		t.Fatalf("expected timeout at hard round limit to be accepted: %v", err)
	}
}

func TestRankRunsLockedMatchesDatabaseLeaderboardRules(t *testing.T) {
	state := &appState{
		runs: []leaderboardEntry{
			{Username: "时间到未清空", DurationMs: defaultRoundLimitMs, MaxSingleSpend: 999_000, FinalBalance: 10_000, EndedBy: "timeout"},
			{Username: "终局清零", DurationMs: 4_500, MaxSingleSpend: 70_000, FinalBalance: 0, EndedBy: "terminal_event"},
			{Username: "清空慢一点", DurationMs: 5_000, MaxSingleSpend: 100_000, FinalBalance: 0, EndedBy: "balance_zero"},
			{Username: "同秒大单", DurationMs: 5_000, MaxSingleSpend: 300_000, FinalBalance: 0, EndedBy: "balance_zero"},
			{Username: "同分先来", DurationMs: 6_000, MaxSingleSpend: 90_000, FinalBalance: 0, EndedBy: "balance_zero"},
			{Username: "同分后来", DurationMs: 6_000, MaxSingleSpend: 90_000, FinalBalance: 0, EndedBy: "balance_zero"},
			{Username: "清空最快", DurationMs: 4_000, MaxSingleSpend: 50_000, FinalBalance: 0, EndedBy: "balance_zero"},
		},
	}

	/*
	 * PostgreSQL 排行榜的 ORDER BY 是：最终余额归零优先，然后用时升序，然后最大单笔降序。
	 * 如果这些公开排序字段都相同，PostgreSQL 会继续按 created_at 和 id 做最终稳定排序；
	 * 内存兜底没有数据库 id，但 state.runs 是按插入顺序 append 的，稳定排序会保留这个
	 * 顺序，效果就和数据库按更早写入的 id 排在前面一致。这样本地无数据库开发时不会看到
	 * 另一套排行榜。终局事件如果把余额清到 0，也属于“余额归零成绩”；但它仍然不会把
	 * endedBy 暴露到公开排行榜 JSON 里，页面只展示名次、用户名、用时和最大单笔。
	 */
	state.rankRunsLocked()

	wantOrder := []string{"清空最快", "终局清零", "同秒大单", "清空慢一点", "同分先来", "同分后来", "时间到未清空"}
	for index, want := range wantOrder {
		got := state.runs[index]
		if got.Username != want {
			t.Fatalf("rank %d = %q, want %q", index+1, got.Username, want)
		}
		if got.Rank != index+1 {
			t.Fatalf("entry %q rank = %d, want %d", got.Username, got.Rank, index+1)
		}
	}
}

func TestSubmitRunMemoryFallbackRejectsDuplicateUsername(t *testing.T) {
	state := &appState{
		usernames: map[string]usernameReservation{"重复用户": {}},
		runs: []leaderboardEntry{
			{Username: "重复用户", DurationMs: 100_000, MaxSingleSpend: 88_000, EndedBy: "balance_zero"},
		},
	}
	body := `{
		"username":"重复用户",
		"durationMs":120000,
		"maxSingleSpend":99000,
		"finalBalance":0,
		"totalSpent":2500000,
		"totalIncome":0,
		"endedBy":"balance_zero",
		"chaosSeed":"retry-seed"
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(body))
	response := httptest.NewRecorder()

	state.handleSubmitRun(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "run_not_accepted") {
		t.Fatalf("expected run_not_accepted response, got %s", response.Body.String())
	}
	if len(state.runs) != 1 {
		t.Fatalf("duplicate submit changed run count to %d", len(state.runs))
	}
}

func TestSubmitRunMemoryFallbackReservesUsernameForDirectSubmit(t *testing.T) {
	state := &appState{
		usernames: map[string]usernameReservation{},
	}

	/*
	 * 数据库路径允许调用方直接提交 /api/runs：submitRunToDatabase 会先补 usernames，
	 * 再写 runs。内存兜底路径也要保持这个行为，否则无数据库开发时，一个已经提交过
	 * 成绩的名字还能被 /api/users/reserve 再次预约，前后端联调看到的规则就会漂移。
	 */
	body, err := json.Marshal(validRunForTest("直接提交"))
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}

	submitRequest := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(string(body)))
	submitResponse := httptest.NewRecorder()
	state.handleSubmitRun(submitResponse, submitRequest)

	if submitResponse.Code != http.StatusCreated {
		t.Fatalf("submit status = %d, want %d; body: %s", submitResponse.Code, http.StatusCreated, submitResponse.Body.String())
	}
	if _, exists := state.usernames["直接提交"]; !exists {
		t.Fatalf("direct submit did not reserve username in memory fallback")
	}

	reserveRequest := httptest.NewRequest(http.MethodPost, "/api/users/reserve", strings.NewReader(`{"username":"直接提交"}`))
	reserveResponseRecorder := httptest.NewRecorder()
	state.handleReserveUser(reserveResponseRecorder, reserveRequest)

	if reserveResponseRecorder.Code != http.StatusOK {
		t.Fatalf("reserve status = %d, want %d; body: %s", reserveResponseRecorder.Code, http.StatusOK, reserveResponseRecorder.Body.String())
	}

	var reserve reserveResponse
	if err := json.Unmarshal(reserveResponseRecorder.Body.Bytes(), &reserve); err != nil {
		t.Fatalf("decode reserve response: %v", err)
	}
	if reserve.Reserved {
		t.Fatalf("reserve.Reserved = true, want false for a username already submitted through /api/runs")
	}
}

func TestHealthReportsFallbackWithoutDatabase(t *testing.T) {
	state := &appState{}
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	state.handleHealth(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload["status"] != "ok" || payload["database"] != "fallback" {
		t.Fatalf("health payload = %+v, want ok fallback", payload)
	}
}

func TestHealthReportsUnavailableWhenDatabasePingFails(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost/crazy_fuckwit_250?sslmode=disable")
	if err != nil {
		t.Fatalf("open closed test database handle: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test database handle: %v", err)
	}
	state := &appState{db: db}
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	/*
	 * 这条测试锁住 /healthz 的真实含义：配置了数据库时，Go 进程存在还不够，
	 * PostgreSQL 当前也必须能被 Ping 通。否则前端恢复探活会误以为真实排行榜、
	 * 内容包和用户名占用都已经可用。
	 */
	state.handleHealth(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}

	var payload apiErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health error: %v", err)
	}
	if payload.Code != "database_unavailable" {
		t.Fatalf("health error code = %q, want database_unavailable", payload.Code)
	}
}

func TestSubmitRunDatabaseFailureReturnsServerError(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost/crazy_fuckwit_250?sslmode=disable")
	if err != nil {
		t.Fatalf("open closed test database handle: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test database handle: %v", err)
	}
	state := &appState{db: db}

	body, err := json.Marshal(validRunForTest("数据库故障"))
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(string(body)))
	response := httptest.NewRecorder()
	state.handleSubmitRun(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "database_error") {
		t.Fatalf("expected database_error response, got %s", response.Body.String())
	}
}

func TestBootstrapDatabaseFailureReturnsServerError(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost/crazy_fuckwit_250?sslmode=disable")
	if err != nil {
		t.Fatalf("open closed test database handle: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test database handle: %v", err)
	}

	state := &appState{db: db}
	request := httptest.NewRequest(http.MethodGet, "/api/content/bootstrap", nil)
	response := httptest.NewRecorder()

	/*
	 * 配置了数据库时，内容包就是正式主线。数据库读取失败不能退回静态内容并返回 200，
	 * 否则前端会以为自己拿到了真实数据库商品、场景和事件，后续算法排查也会被误导。
	 */
	state.handleBootstrap(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "database_error") {
		t.Fatalf("expected database_error response, got %s", response.Body.String())
	}
}

func TestDecodeJSONRejectsTrailingPayload(t *testing.T) {
	body := `{"username":"正常用户"}{"username":"尾随用户"}`
	request := httptest.NewRequest(http.MethodPost, "/api/users/reserve", strings.NewReader(body))
	response := httptest.NewRecorder()

	var payload reserveRequest
	if decodeJSON(response, request, &payload) {
		t.Fatalf("expected decodeJSON to reject a second JSON object")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "invalid_json") {
		t.Fatalf("expected invalid_json response, got %s", response.Body.String())
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	body := `{"username":"正常用户","settlementStats":{}}`
	request := httptest.NewRequest(http.MethodPost, "/api/users/reserve", strings.NewReader(body))
	response := httptest.NewRecorder()

	var payload reserveRequest
	if decodeJSON(response, request, &payload) {
		t.Fatalf("expected decodeJSON to reject unknown fields")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestLeaderboardEntryDoesNotExposeInternalEndReason(t *testing.T) {
	body, err := json.Marshal(leaderboardEntry{
		Rank:           1,
		Username:       "隐藏字段",
		DurationMs:     1_200,
		MaxSingleSpend: 50_000,
		EndedBy:        "balance_zero",
	})
	if err != nil {
		t.Fatalf("marshal leaderboard entry: %v", err)
	}

	if strings.Contains(string(body), "endedBy") || strings.Contains(string(body), "finalBalance") || strings.Contains(string(body), "balance_zero") {
		t.Fatalf("leaderboard JSON leaked internal end reason: %s", string(body))
	}
}
