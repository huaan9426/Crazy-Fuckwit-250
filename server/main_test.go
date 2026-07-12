package main

import (
	"encoding/json"
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
		TotalSpent:     defaultInitialBalance,
		TotalIncome:    0,
		EndedBy:        "balance_zero",
		ChaosSeed:      "test-seed",
		ContentVersion: testContentVersion,
	}
}

func TestContentVersionTracksContentChanges(t *testing.T) {
	first := withContentVersion(bootstrapContent())
	second := withContentVersion(bootstrapContent())
	if !isGeneratedContentVersion(first.Config.ContentVersion) {
		t.Fatalf("content version = %q, want generated fingerprint", first.Config.ContentVersion)
	}
	if first.Config.ContentVersion != second.Config.ContentVersion {
		t.Fatalf("same content produced different versions: %q and %q", first.Config.ContentVersion, second.Config.ContentVersion)
	}

	second.Items[0].Price += 1
	second = withContentVersion(second)
	if first.Config.ContentVersion == second.Config.ContentVersion {
		t.Fatalf("content version did not change after item price changed")
	}
}

func TestContentVersionNormalization(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		normalized string
		optional   string
	}{
		{name: "empty", value: "", normalized: unknownContentVersion, optional: ""},
		{name: "generated", value: " " + testContentVersion + " ", normalized: testContentVersion, optional: testContentVersion},
		{name: "unknown", value: unknownContentVersion, normalized: unknownContentVersion, optional: unknownContentVersion},
		{name: "manual", value: "release-1", normalized: unknownContentVersion, optional: ""},
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

func TestMemoryFallbackKeepsRepresentativeContent(t *testing.T) {
	bootstrap := bootstrapContent()
	seenTiers := make(map[string]bool)
	lowestSpendPrice := int64(0)
	for _, item := range bootstrap.Items {
		seenTiers[item.Tier] = true
		if item.Tier != "income" && (lowestSpendPrice == 0 || item.Price < lowestSpendPrice) {
			lowestSpendPrice = item.Price
		}
	}

	for _, tier := range []string{"coin", "small", "daily", "income", "shock"} {
		if !seenTiers[tier] {
			t.Fatalf("memory fallback is missing representative tier %q", tier)
		}
	}
	if lowestSpendPrice != 1 {
		t.Fatalf("memory fallback lowest spend price = %d, want 1", lowestSpendPrice)
	}
}

func TestUsernameNormalizationAndValidation(t *testing.T) {
	if got := normalizeUsername("  今晚   花完  "); got != "今晚 花完" {
		t.Fatalf("normalizeUsername returned %q", got)
	}

	for _, username := range []string{"正常用户", "今晚 花完", "ab"} {
		if !validUsername(username) {
			t.Fatalf("valid username %q was rejected", username)
		}
	}
	for _, username := range []string{"a", "带/斜杠", "前后 ", strings.Repeat("长", 17)} {
		if validUsername(username) {
			t.Fatalf("invalid username %q was accepted", username)
		}
	}
}

func TestMemoryReservationLifecycle(t *testing.T) {
	state := &appState{usernames: make(map[string]usernameReservation)}
	now := time.Now()

	reserved, token, err := state.reserveUsernameInMemory("预约用户", "", now)
	if err != nil || !reserved || token == "" {
		t.Fatalf("initial reservation failed: reserved=%v token=%q err=%v", reserved, token, err)
	}

	renewed, renewedToken, err := state.reserveUsernameInMemory("预约用户", token, now.Add(time.Minute))
	if err != nil || !renewed || renewedToken != token {
		t.Fatalf("matching token did not renew reservation: renewed=%v token=%q err=%v", renewed, renewedToken, err)
	}

	reserved, _, err = state.reserveUsernameInMemory("预约用户", "wrong-token", now.Add(2*time.Minute))
	if err != nil || reserved {
		t.Fatalf("wrong token should not take an active reservation: reserved=%v err=%v", reserved, err)
	}

	reserved, replacementToken, err := state.reserveUsernameInMemory("预约用户", "", now.Add(usernameReservationTTL+time.Minute))
	if err != nil || !reserved || replacementToken == "" || replacementToken == token {
		t.Fatalf("expired reservation was not replaced: reserved=%v token=%q err=%v", reserved, replacementToken, err)
	}
}

func TestValidateRunCoreContracts(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*runSubmission)
		wantErr bool
	}{
		{name: "valid balance zero"},
		{
			name: "money does not balance",
			mutate: func(run *runSubmission) {
				run.FinalBalance = 1
			},
			wantErr: true,
		},
		{
			name: "max spend exceeds total",
			mutate: func(run *runSubmission) {
				run.MaxSingleSpend = run.TotalSpent + 1
			},
			wantErr: true,
		},
		{
			name: "timeout before hard limit",
			mutate: func(run *runSubmission) {
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs - 1
				run.FinalBalance = 10_000
				run.TotalSpent = defaultInitialBalance - run.FinalBalance
			},
			wantErr: true,
		},
		{
			name: "valid timeout",
			mutate: func(run *runSubmission) {
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				run.FinalBalance = 10_000
				run.TotalSpent = defaultInitialBalance - run.FinalBalance
			},
		},
		{
			name: "terminal event missing details",
			mutate: func(run *runSubmission) {
				run.EndedBy = "terminal_event"
			},
			wantErr: true,
		},
		{
			name: "valid terminal event",
			mutate: func(run *runSubmission) {
				run.EndedBy = "terminal_event"
				run.EndingID = "ending-test"
				run.EndingTitle = "测试终局"
				run.EndingDetail = "测试终局说明"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := validRunForTest("成绩用户")
			if test.mutate != nil {
				test.mutate(&run)
			}
			err := validateRun(run)
			if test.wantErr && err == nil {
				t.Fatalf("invalid run was accepted: %+v", run)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("valid run was rejected: %v", err)
			}
		})
	}
}

func TestMemoryLeaderboardRanking(t *testing.T) {
	state := &appState{runs: []leaderboardEntry{
		{Username: "未清空", DurationMs: 80_000, MaxSingleSpend: 90_000, FinalBalance: 1},
		{Username: "清空较慢", DurationMs: 120_000, MaxSingleSpend: 40_000, FinalBalance: 0},
		{Username: "清空较快", DurationMs: 90_000, MaxSingleSpend: 30_000, FinalBalance: 0},
	}}
	state.rankRunsLocked()

	want := []string{"清空较快", "清空较慢", "未清空"}
	for index, username := range want {
		if state.runs[index].Username != username || state.runs[index].Rank != index+1 {
			t.Fatalf("rank %d = %+v, want username %q", index+1, state.runs[index], username)
		}
	}
}

func TestDecodeJSONRejectsUnknownAndTrailingFields(t *testing.T) {
	for _, body := range []string{
		`{"username":"测试用户","unknown":true}`,
		`{"username":"测试用户"}{"username":"第二个对象"}`,
	} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/api/users/reserve", strings.NewReader(body))
			response := httptest.NewRecorder()
			var payload reserveRequest
			if decodeJSON(response, request, &payload) {
				t.Fatalf("invalid JSON payload was accepted: %s", body)
			}
			if response.Code != 400 {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
}

func TestLeaderboardEntryHidesInternalFields(t *testing.T) {
	encoded, err := json.Marshal(leaderboardEntry{
		Rank:           1,
		Username:       "测试用户",
		DurationMs:     120_000,
		MaxSingleSpend: 42_000,
		FinalBalance:   0,
		EndedBy:        "terminal_event",
	})
	if err != nil {
		t.Fatalf("marshal leaderboard entry: %v", err)
	}
	if strings.Contains(string(encoded), "finalBalance") || strings.Contains(string(encoded), "endedBy") {
		t.Fatalf("leaderboard leaked internal fields: %s", encoded)
	}
}
