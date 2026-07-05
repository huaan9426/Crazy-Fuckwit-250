package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func openIntegrationDatabase(t *testing.T) *appState {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL != "" {
		t.Setenv("DATABASE_URL", databaseURL)
	} else if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Skip("set DATABASE_URL or TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	db, err := openConfiguredDatabase()
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	if db == nil {
		t.Skip("DATABASE_URL is empty")
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close integration database: %v", err)
		}
	})

	return &appState{db: db}
}

var integrationUsernameCounter uint64

func integrationUsername(prefix string) string {
	return prefix + strconv.FormatUint(atomic.AddUint64(&integrationUsernameCounter, 1), 36)
}

func integrationContentVersion() string {
	return fmt.Sprintf("sha256:%064x", atomic.AddUint64(&integrationUsernameCounter, 1))
}

func cleanupIntegrationUsers(t *testing.T, state *appState, prefix string) {
	t.Helper()

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cancel()

		if _, err := state.db.ExecContext(ctx, `DELETE FROM runs WHERE username LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration runs: %v", err)
		}
		if _, err := state.db.ExecContext(ctx, `DELETE FROM usernames WHERE username LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration usernames: %v", err)
		}
	}

	cleanup()
	t.Cleanup(cleanup)
}

func cleanupIntegrationContent(t *testing.T, state *appState, prefix string) {
	t.Helper()

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cancel()

		if _, err := state.db.ExecContext(ctx, `DELETE FROM content_items WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration content items: %v", err)
		}
		if _, err := state.db.ExecContext(ctx, `DELETE FROM content_events WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration content events: %v", err)
		}
		if _, err := state.db.ExecContext(ctx, `DELETE FROM content_endings WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration content endings: %v", err)
		}
		if _, err := state.db.ExecContext(ctx, `DELETE FROM content_scenes WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration content scenes: %v", err)
		}
	}

	cleanup()
	t.Cleanup(cleanup)
}

func integrationRun(username string, endedBy string, durationMs int64, maxSingleSpend int64) runSubmission {
	run := validRunForTest(username)
	run.EndedBy = endedBy
	run.DurationMs = durationMs
	run.MaxSingleSpend = maxSingleSpend
	if endedBy != "balance_zero" {
		run.FinalBalance = 10_000
		run.TotalSpent = defaultInitialBalance + run.TotalIncome - run.FinalBalance
	}

	return run
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func TestDatabaseBootstrapFiltersContentByDefaultMode(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-other-mode"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	otherMode := `["other-mode"]`
	inserts := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name: "scene",
			query: `
				INSERT INTO content_scenes (
					id, name, entry_cost, duration_sec, min_balance, rarity, risk_level, item_tags, event_tags, modes, sort_order, active
				)
				VALUES ($1, '其他模式场景', 0, 35, 0, 'common', 1, '[]'::jsonb, '[]'::jsonb, $2::jsonb, 999001, true)
			`,
			args: []any{prefix + "-scene", otherMode},
		},
		{
			name: "item",
			query: `
				INSERT INTO content_items (
					id, name, category, scene_id, price, tier, weight, min_balance, modes, tags, flavor, sort_order, active
				)
				VALUES ($1, '其他模式商品', '测试', NULL, 100, 'daily', 1, 0, $2::jsonb, '["test"]'::jsonb, '不应出现在默认模式。', 999002, true)
			`,
			args: []any{prefix + "-item", otherMode},
		},
		{
			name: "event",
			query: `
				INSERT INTO content_events (
					id, title, description, delta, probability, cooldown_sec, tags, modes, settlement_tag, sort_order, active
				)
				VALUES ($1, '其他模式事件', '不应出现在默认模式。', -1, 0.1, 1, '["test"]'::jsonb, $2::jsonb, '测试', 999003, true)
			`,
			args: []any{prefix + "-event", otherMode},
		},
		{
			name: "ending",
			query: `
				INSERT INTO content_endings (
					id, title, description, probability, min_elapsed_ms, min_risk_level, balance_effect, tags, modes, settlement_tag, sort_order, active
				)
				VALUES ($1, '其他模式终局', '不应出现在默认模式。', 0.001, 0, 1, 'none', '["test"]'::jsonb, $2::jsonb, '测试', 999004, true)
			`,
			args: []any{prefix + "-ending", otherMode},
		},
	}
	for _, insert := range inserts {
		if _, err := state.db.ExecContext(ctx, insert.query, insert.args...); err != nil {
			t.Fatalf("insert other-mode %s: %v", insert.name, err)
		}
	}

	bootstrap, err := state.loadBootstrapFromDatabase(ctx)
	if err != nil {
		t.Fatalf("load bootstrap from database: %v", err)
	}

	for _, scene := range bootstrap.Scenes {
		if strings.HasPrefix(scene.ID, prefix) {
			t.Fatalf("other-mode scene %q leaked into default bootstrap", scene.ID)
		}
	}
	for _, item := range bootstrap.Items {
		if strings.HasPrefix(item.ID, prefix) {
			t.Fatalf("other-mode item %q leaked into default bootstrap", item.ID)
		}
	}
	for _, event := range bootstrap.Events {
		if strings.HasPrefix(event.ID, prefix) {
			t.Fatalf("other-mode event %q leaked into default bootstrap", event.ID)
		}
	}
	for _, ending := range bootstrap.Endings {
		if strings.HasPrefix(ending.ID, prefix) {
			t.Fatalf("other-mode ending %q leaked into default bootstrap", ending.ID)
		}
	}
}

func TestDatabaseRejectsMalformedModeJSONShapeAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-mode-json"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * PostgreSQL 的 JSONB `?` 操作符既能检查数组元素，也能检查对象 key。这里故意把 modes
	 * 写成对象。如果表级约束缺失，SQL 的 `modes ? 'chaos-life'` 会选中它，然后 Go API
	 * 只能在读取内容包时返回 500。现在数据库写入阶段就应该拒绝这种形态，避免坏内容进入
	 * 后续运行时链路。
	 */
	if _, err := state.db.ExecContext(ctx, `
		INSERT INTO content_items (
			id, name, category, scene_id, price, tier, weight, min_balance, modes, tags, flavor, sort_order, active
		)
		VALUES (
			$1 || '-item', '坏 modes 商品', '测试', NULL, 100, 'daily', 1, 0,
			'{"chaos-life": true}'::jsonb, '["daily"]'::jsonb, '这条记录应被运行时校验拒绝。', 999998, true
		)
	`, prefix); err == nil {
		t.Fatalf("expected malformed modes JSON shape to be rejected at database write time")
	}
}

func TestDatabaseRejectsUnsafeSceneRuntimeFieldsAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-scene-runtime"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * 场景字段会直接影响前端阶段节奏。duration_sec 现在只是内容说明，真实阶段时钟来自
	 * balanceTuning.stageDurationMs，所以两边必须保持同一个 35 秒值；entry_cost 和
	 * min_balance 则是金额门槛，负数会被前端当成“不收费”或“永远满足条件”。这里用真实
	 * PostgreSQL 约束确认坏场景不能先写入内容表。
	 */
	badSceneTests := []struct {
		suffix      string
		entryCost   int64
		durationSec int64
		minBalance  int64
	}{
		{suffix: "negative-entry", entryCost: -1, durationSec: 35, minBalance: 0},
		{suffix: "duration-drift", entryCost: 0, durationSec: 18, minBalance: 0},
		{suffix: "negative-min-balance", entryCost: 0, durationSec: 35, minBalance: -1},
	}
	for _, test := range badSceneTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_scenes (
				id, name, entry_cost, duration_sec, min_balance, rarity, risk_level, item_tags, event_tags, modes, sort_order, active
			)
			VALUES (
				$1, '坏场景', $2, $3, $4, 'common', 1,
				'["daily"]'::jsonb, '["fee"]'::jsonb, '["chaos-life"]'::jsonb, 999992, true
			)
		`, prefix+"-"+test.suffix, test.entryCost, test.durationSec, test.minBalance); err == nil {
			t.Fatalf("expected unsafe scene runtime fields %s to be rejected at database write time", test.suffix)
		}
	}
}

func TestDatabaseRejectsNonPositiveMaxBuyAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-max-buy"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * max_buy 是数据库内容包和前端次数限制之间的协议字段。NULL 表示不限次数，正数表示
	 * 本局最多可以买几次。0 或负数没有独立玩法含义，如果允许写入，前端会把它兜底成
	 * 不限次数，反而放大坏数据的影响。因此这里用真实 PostgreSQL 约束确认坏值在写入
	 * 阶段就被拒绝，而不是等到浏览器运行时再靠兜底逻辑猜测。
	 */
	badMaxBuyTests := []struct {
		suffix string
		value  int64
	}{
		{suffix: "zero", value: 0},
		{suffix: "negative", value: -1},
	}
	for _, test := range badMaxBuyTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_items (
				id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, modes, tags, flavor, sort_order, active
			)
			VALUES (
				$1, '坏次数商品', '测试', NULL, 100, 'daily', $2, false, 1, 0,
				'["chaos-life"]'::jsonb, '["daily"]'::jsonb, '这条记录应被数据库写入约束拒绝。', 999997, true
			)
		`, prefix+"-"+test.suffix+"-item", test.value); err == nil {
			t.Fatalf("expected max_buy=%d to be rejected at database write time", test.value)
		}
	}
}

func TestDatabaseRejectsNonPositiveItemPriceAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-price"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * price 是商品卡进入前端金额算法的基础字段。消费卡靠它扣余额，income 卡靠它加余额，
	 * 所以 0 或负数都没有稳定玩法含义。把这个约束放在数据库层，可以防止后台内容编辑或
	 * seed 修改把零价卡写进去，然后让前端发出“可点击但不推进余额”的卡。
	 */
	badPriceTests := []struct {
		suffix string
		value  int64
	}{
		{suffix: "zero", value: 0},
		{suffix: "negative", value: -1},
	}
	for _, test := range badPriceTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_items (
				id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, modes, tags, flavor, sort_order, active
			)
			VALUES (
				$1, '坏价格商品', '测试', NULL, $2, 'daily', NULL, false, 1, 0,
				'["chaos-life"]'::jsonb, '["daily"]'::jsonb, '这条记录应被数据库写入约束拒绝。', 999996, true
			)
		`, prefix+"-"+test.suffix+"-item", test.value); err == nil {
			t.Fatalf("expected price=%d to be rejected at database write time", test.value)
		}
	}
}

func TestDatabaseRejectsUnsafeItemRuntimeFieldsAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-item-runtime"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * weight 和 min_balance 不是普通展示字段。weight 会进入前端抽牌权重，min_balance 会
	 * 决定商品何时进入候选池；如果旧表没有继承后来写在 CREATE TABLE 里的 CHECK，直接
	 * SQL 仍可能把非正权重或负余额门槛写进去。这个测试确认 schema 里的 ALTER 约束会在
	 * 写入阶段拦住坏数据，而不是等 Go 内容包读取时才发现。
	 */
	tests := []struct {
		name       string
		weight     int64
		minBalance int64
	}{
		{name: "zero-weight", weight: 0, minBalance: 0},
		{name: "negative-weight", weight: -1, minBalance: 0},
		{name: "negative-min-balance", weight: 1, minBalance: -1},
	}
	for _, test := range tests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_items (
				id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, modes, tags, flavor, sort_order, active
			)
			VALUES (
				$1, '坏抽牌字段商品', '测试', NULL, 100, 'daily', NULL, false, $2, $3,
				'["chaos-life"]'::jsonb, '["daily"]'::jsonb, '这条记录应被数据库写入约束拒绝。', 999995, true
			)
		`, prefix+"-"+test.name+"-item", test.weight, test.minBalance); err == nil {
			t.Fatalf("expected item runtime fields weight=%d min_balance=%d to be rejected at database write time", test.weight, test.minBalance)
		}
	}
}

func TestDatabaseRejectsUnsafeEventProbabilityAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-event-probability"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * probability 是前端挑选混沌事件时使用的权重，不是简单的展示字段。Go 内容包校验已经
	 * 要求它必须大于 0 且不能高于普通事件上限；如果数据库允许 0 或过高值写入，API 后续
	 * 会在读取内容包时才失败。这里把同一条规则放进表约束，保证坏内容不能先落库。
	 */
	badProbabilityTests := []struct {
		suffix string
		value  float64
	}{
		{suffix: "zero", value: 0},
		{suffix: "too-high", value: maximumChaosEventProbability + 0.01},
	}
	for _, test := range badProbabilityTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_events (
				id, title, description, delta, probability, cooldown_sec, tags, modes, settlement_tag, sort_order, active
			)
			VALUES (
				$1, '坏概率事件', '这条记录应被数据库写入约束拒绝。', -100, $2, 1,
				'["daily"]'::jsonb, '["chaos-life"]'::jsonb, '坏概率', 999995, true
			)
		`, prefix+"-"+test.suffix+"-event", test.value); err == nil {
			t.Fatalf("expected event probability %.4f to be rejected at database write time", test.value)
		}
	}
}

func TestDatabaseRejectsUnsafeEventDeltaAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-event-delta"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * delta 是混沌事件真正改变余额的金额字段。Go 内容包校验把单次事件影响限制在
	 * 初始余额的 1/25，也就是当前 100000 元以内，避免一个随机事件直接盖过整局消费节奏。
	 * 数据库也需要同一条边界，否则 seed 或后台内容可以先写入超大扣款/返钱事件，再让
	 * `/api/content/bootstrap` 在读取内容包时失败。
	 */
	badDeltaTests := []struct {
		suffix string
		value  int64
	}{
		{suffix: "spend-too-large", value: -100001},
		{suffix: "income-too-large", value: 100001},
	}
	for _, test := range badDeltaTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_events (
				id, title, description, delta, probability, cooldown_sec, tags, modes, settlement_tag, sort_order, active
			)
			VALUES (
				$1, '坏金额事件', '这条记录应被数据库写入约束拒绝。', $2, 0.1, 1,
				'["daily"]'::jsonb, '["chaos-life"]'::jsonb, '坏金额', 999990, true
			)
		`, prefix+"-"+test.suffix+"-event", test.value); err == nil {
			t.Fatalf("expected event delta %d to be rejected at database write time", test.value)
		}
	}
}

func TestDatabaseRejectsUnsafeTerminalProbabilityAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-ending-probability"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * 终局事件的 probability 比普通混沌事件更低，因为它会提前结束一局并写入结算反馈。
	 * 如果数据库允许 0，active 的终局事件会变成永远不会触发的假内容；如果允许过高值，
	 * 玩家会频繁被提前打断。这个测试用真实 PostgreSQL 约束确认终局事件概率和 Go 校验
	 * 保持同一边界。
	 */
	badProbabilityTests := []struct {
		suffix string
		value  float64
	}{
		{suffix: "zero", value: 0},
		{suffix: "too-high", value: maxTerminalEventProbability + 0.001},
	}
	for _, test := range badProbabilityTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_endings (
				id, title, description, probability, min_elapsed_ms, min_risk_level, balance_effect, tags, modes, settlement_tag, sort_order, active
			)
			VALUES (
				$1, '坏概率终局', '这条记录应被数据库写入约束拒绝。', $2, 1000, 1, 'none',
				'["daily"]'::jsonb, '["chaos-life"]'::jsonb, '坏终局', 999994, true
			)
		`, prefix+"-"+test.suffix+"-ending", test.value); err == nil {
			t.Fatalf("expected terminal probability %.4f to be rejected at database write time", test.value)
		}
	}
}

func TestDatabaseRejectsUnsafeTerminalTriggerFieldsAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-ending-trigger"
	cleanupIntegrationContent(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * Go 内容包校验会拒绝这些坏终局字段，但 PostgreSQL 也要挡住直接写入。否则后续维护
	 * 脚本或手工 seed 可以先把负时间、不可触发余额、非法风险等级或未知 balance_effect
	 * 写进库里，再让前端拿到一条会过早触发或结算语义不清的终局事件。
	 */
	tests := []struct {
		name          string
		minElapsedMs  int64
		maxBalanceSQL string
		minRiskLevel  int64
		balanceEffect string
	}{
		{name: "negative elapsed", minElapsedMs: -1, maxBalanceSQL: "NULL", minRiskLevel: 1, balanceEffect: "none"},
		{name: "zero max balance", minElapsedMs: 1000, maxBalanceSQL: "0", minRiskLevel: 1, balanceEffect: "none"},
		{name: "negative max balance", minElapsedMs: 1000, maxBalanceSQL: "-1", minRiskLevel: 1, balanceEffect: "none"},
		{name: "risk too low", minElapsedMs: 1000, maxBalanceSQL: "NULL", minRiskLevel: 0, balanceEffect: "none"},
		{name: "risk too high", minElapsedMs: 1000, maxBalanceSQL: "NULL", minRiskLevel: 6, balanceEffect: "none"},
		{name: "unsupported balance effect", minElapsedMs: 1000, maxBalanceSQL: "NULL", minRiskLevel: 1, balanceEffect: "refund"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := state.db.ExecContext(ctx, `
				INSERT INTO content_endings (
					id, title, description, probability, min_elapsed_ms, max_balance, min_risk_level, balance_effect,
					tags, modes, settlement_tag, sort_order, active
				)
				VALUES (
					$1, '坏触发字段终局', '这条记录应被数据库写入约束拒绝。', 0.0005, $2, `+test.maxBalanceSQL+`, $3, $4,
					'["daily"]'::jsonb, '["chaos-life"]'::jsonb, '坏终局', 999993, true
				)
			`, prefix+"-"+strings.ReplaceAll(test.name, " ", "-"), test.minElapsedMs, test.minRiskLevel, test.balanceEffect)
			if err == nil {
				t.Fatalf("expected unsafe terminal trigger fields for %s to be rejected at database write time", test.name)
			}
		})
	}
}

func TestDatabaseRejectsUnsafeStatusTuningAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-status-tuning"

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cancel()

		if _, err := state.db.ExecContext(ctx, `DELETE FROM content_statuses WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration status effects: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * 状态效果字段直接进入前端节奏算法：duration_sec 决定状态持续多久，
	 * item_refresh_multiplier 决定货架刷新速度，high_price_multiplier 决定大额卡权重，
	 * event_multiplier 决定混沌事件触发概率。Go 内容包校验已经限制这些数值落在可玩的
	 * 区间内，数据库也必须使用同一组边界，否则后台或 seed 误写入后会先污染内容表，
	 * 再让 API 读取内容包时失败。
	 */
	badStatusTests := []struct {
		suffix                string
		durationSec           int64
		itemRefreshMultiplier float64
		highPriceMultiplier   float64
		eventMultiplier       float64
	}{
		{suffix: "duration-short", durationSec: minimumStatusDurationSec - 1, itemRefreshMultiplier: 1, highPriceMultiplier: 1, eventMultiplier: 1},
		{suffix: "duration-long", durationSec: maximumStatusDurationSec + 1, itemRefreshMultiplier: 1, highPriceMultiplier: 1, eventMultiplier: 1},
		{suffix: "refresh-low", durationSec: minimumStatusDurationSec, itemRefreshMultiplier: 0.49, highPriceMultiplier: 1, eventMultiplier: 1},
		{suffix: "refresh-high", durationSec: minimumStatusDurationSec, itemRefreshMultiplier: 1.81, highPriceMultiplier: 1, eventMultiplier: 1},
		{suffix: "high-price-low", durationSec: minimumStatusDurationSec, itemRefreshMultiplier: 1, highPriceMultiplier: 0.49, eventMultiplier: 1},
		{suffix: "high-price-high", durationSec: minimumStatusDurationSec, itemRefreshMultiplier: 1, highPriceMultiplier: 2.01, eventMultiplier: 1},
		{suffix: "event-low", durationSec: minimumStatusDurationSec, itemRefreshMultiplier: 1, highPriceMultiplier: 1, eventMultiplier: 0.49},
		{suffix: "event-high", durationSec: minimumStatusDurationSec, itemRefreshMultiplier: 1, highPriceMultiplier: 1, eventMultiplier: 1.81},
	}
	for _, test := range badStatusTests {
		if _, err := state.db.ExecContext(ctx, `
			INSERT INTO content_statuses (
				id, name, duration_sec, item_refresh_multiplier, high_price_multiplier, event_multiplier, tags, description, sort_order, active
			)
			VALUES (
				$1, '坏状态', $2, $3, $4, $5,
				'["daily"]'::jsonb, '这条记录应被数据库写入约束拒绝。', 999993, true
			)
		`,
			prefix+"-"+test.suffix,
			test.durationSec,
			test.itemRefreshMultiplier,
			test.highPriceMultiplier,
			test.eventMultiplier,
		); err == nil {
			t.Fatalf("expected unsafe status tuning %s to be rejected at database write time", test.suffix)
		}
	}
}

func TestDatabaseRejectsUnsupportedAudioLicenseAtWrite(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-bad-audio-license"

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cancel()

		if _, err := state.db.ExecContext(ctx, `DELETE FROM audio_tracks WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup integration audio tracks: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * 音轨 license 会作为内容包字段发给前端。前端类型只接受 CC0、MIT 和 custom，
	 * 分别代表公共领域、宽松开源和项目自有或另行授权。数据库如果允许任意字符串，
	 * 后续接入真实音乐文件时，素材授权信息就会在 API 边界失真。
	 */
	if _, err := state.db.ExecContext(ctx, `
		INSERT INTO audio_tracks (
			id, title, mood, src, license, source_url, sort_order, active
		)
		VALUES (
			$1 || '-audio', '坏授权音轨', 'rush', '', 'unknown', '', 999991, true
		)
	`, prefix); err == nil {
		t.Fatalf("expected unsupported audio license to be rejected at database write time")
	}
}

func TestDatabaseSeedDeactivatesRemovedCurrentModeContent(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "integration-stale-seed"

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cleanupCancel()

		if _, err := state.db.ExecContext(cleanupCtx, `DELETE FROM content_items WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup stale content items: %v", err)
		}
		if _, err := state.db.ExecContext(cleanupCtx, `DELETE FROM content_events WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup stale content events: %v", err)
		}
		if _, err := state.db.ExecContext(cleanupCtx, `DELETE FROM content_endings WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup stale content endings: %v", err)
		}
		if _, err := state.db.ExecContext(cleanupCtx, `DELETE FROM content_scenes WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup stale content scenes: %v", err)
		}
		if _, err := state.db.ExecContext(cleanupCtx, `DELETE FROM content_statuses WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup stale content statuses: %v", err)
		}
		if _, err := state.db.ExecContext(cleanupCtx, `DELETE FROM audio_tracks WHERE id LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup stale audio tracks: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	/*
	 * 这个测试模拟“上一版 seed 里有这些内容，下一版 seed 已经删除或重命名了它们”的情况。
	 * 如果 seed 只做 INSERT ... ON CONFLICT UPDATE，旧行会继续 active=true，前端内容包就会
	 * 把已经不在代码里的旧卡、旧事件或旧状态继续交给抽卡算法。首版没有后台内容管理页面，
	 * 所以当前 chaos-life 模式应以 seed.sql 为事实源：重新初始化后，不在 seed 里的旧内容
	 * 必须失活，但真实 seed 内容仍要能通过 loadBootstrapFromDatabase 正常加载。
	 */
	staleInserts := []struct {
		name  string
		query string
	}{
		{
			name: "scene",
			query: `
				INSERT INTO content_scenes (
					id, name, entry_cost, duration_sec, min_balance, rarity, risk_level, item_tags, event_tags, modes, sort_order, active
				) VALUES (
					$1 || '-scene', '旧场景', 0, 35, 0, 'common', 2, '["daily"]'::jsonb, '["refund"]'::jsonb, '["chaos-life"]'::jsonb, 999001, true
				)
			`,
		},
		{
			name: "item",
			query: `
				INSERT INTO content_items (
					id, name, category, scene_id, price, tier, weight, min_balance, modes, tags, flavor, sort_order, active
				) VALUES (
					$1 || '-item', '旧商品', '旧分类', $1 || '-scene', 188, 'daily', 1, 0, '["chaos-life"]'::jsonb, '["daily"]'::jsonb, '旧商品不应该继续出现。', 999002, true
				)
			`,
		},
		{
			name: "event",
			query: `
				INSERT INTO content_events (
					id, title, description, delta, probability, cooldown_sec, tags, modes, settlement_tag, sort_order, active
				) VALUES (
					$1 || '-event', '旧事件', '旧事件不应该继续出现。', -100, 0.1, 1, '["daily"]'::jsonb, '["chaos-life"]'::jsonb, '旧事件', 999003, true
				)
			`,
		},
		{
			name: "ending",
			query: `
				INSERT INTO content_endings (
					id, title, description, probability, min_elapsed_ms, min_risk_level, balance_effect, tags, modes, settlement_tag, sort_order, active
				) VALUES (
					$1 || '-ending', '旧终局', '旧终局不应该继续出现。', 0.0001, 1000, 1, 'none', '["daily"]'::jsonb, '["chaos-life"]'::jsonb, '旧终局', 999004, true
				)
			`,
		},
		{
			name: "status",
			query: `
				INSERT INTO content_statuses (
					id, name, duration_sec, item_refresh_multiplier, high_price_multiplier, event_multiplier, tags, description, sort_order, active
				) VALUES (
					$1 || '-status', '旧状态', 10, 1, 1, 1, '["daily"]'::jsonb, '旧状态不应该继续出现。', 999005, true
				)
			`,
		},
		{
			name: "audio",
			query: `
				INSERT INTO audio_tracks (
					id, title, mood, src, license, source_url, sort_order, active
				) VALUES (
					$1 || '-audio', '旧音轨', 'rush', '', 'custom', '', 999006, true
				)
			`,
		},
	}
	for _, insert := range staleInserts {
		if _, err := state.db.ExecContext(ctx, insert.query, prefix); err != nil {
			t.Fatalf("insert stale seed %s: %v", insert.name, err)
		}
	}

	if err := initializeDatabase(ctx, state.db); err != nil {
		t.Fatalf("reinitialize database: %v", err)
	}

	tables := []string{"content_scenes", "content_items", "content_events", "content_endings", "content_statuses", "audio_tracks"}
	for _, table := range tables {
		var activeCount int
		if err := state.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE id LIKE $1 AND active = true`, prefix+"%").Scan(&activeCount); err != nil {
			t.Fatalf("count active stale rows in %s: %v", table, err)
		}
		if activeCount != 0 {
			t.Fatalf("%s still has %d active stale seed rows", table, activeCount)
		}
	}

	if _, err := state.loadBootstrapFromDatabase(ctx); err != nil {
		t.Fatalf("load bootstrap after seed deactivation: %v", err)
	}
}

func TestDatabaseSeedContentMeetsDevelopmentPlanScale(t *testing.T) {
	state := openIntegrationDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	bootstrap, err := state.loadBootstrapFromDatabase(ctx)
	if err != nil {
		t.Fatalf("load bootstrap from database: %v", err)
	}

	minimums := []struct {
		name string
		got  int
		want int
	}{
		{name: "items", got: len(bootstrap.Items), want: minimumBootstrapItems},
		{name: "scenes", got: len(bootstrap.Scenes), want: minimumBootstrapScenes},
		{name: "events", got: len(bootstrap.Events), want: minimumBootstrapEvents},
		{name: "statuses", got: len(bootstrap.Statuses), want: minimumBootstrapStatuses},
		{name: "endings", got: len(bootstrap.Endings), want: minimumBootstrapEndings},
		{name: "audio tracks", got: len(bootstrap.AudioTracks), want: minimumBootstrapAudioTracks},
	}
	for _, minimum := range minimums {
		if minimum.got < minimum.want {
			t.Fatalf("%s count = %d, want at least %d", minimum.name, minimum.got, minimum.want)
		}
	}

	/*
	 * 开发计划把状态效果定义成一组明确的情绪和节奏状态。这里检查的是用户能在真实
	 * 数据库内容里抽到这些状态，而不是只检查数量。只看数量会掩盖“12 个状态还在，
	 * 但计划里的某个状态被别的实验状态替换掉”的问题，前端算法也就无法按预期表达
	 * 低落、上头、攀比、回光返照这些阶段感。
	 */
	requiredStatusNames := []string{"生气", "低落", "上头", "攀比", "焦虑囤货", "购物麻木", "报复消费", "心悸", "疲劳", "好运", "倒霉", "回光返照"}
	seenStatusNames := make(map[string]struct{})
	lowMoodHasSceneReductionTag := false
	for _, effect := range bootstrap.Statuses {
		seenStatusNames[effect.Name] = struct{}{}
		if effect.Name == "低落" && stringSliceContains(effect.Tags, "low-mood") {
			lowMoodHasSceneReductionTag = true
		}
	}
	for _, name := range requiredStatusNames {
		if _, exists := seenStatusNames[name]; !exists {
			t.Fatalf("database bootstrap is missing development-plan status %q", name)
		}
	}
	if !lowMoodHasSceneReductionTag {
		t.Fatalf("database bootstrap low-mood status must include low-mood tag for frontend scene reduction")
	}

	/*
	 * 开发计划要求商品覆盖绝大多数消费场景和各个价位。这里不检查文案内容，
	 * 只锁住最容易被误删的结构性证据：数据库返回的商品至少要覆盖从零钱、小额、
	 * 日常、普通溢价、大件、重额、冲击高价到反向进账这些主要 tier。这样以后有人
	 * 调整 seed 时，不能不小心把找零阶段、收入卡或高额消费层删掉而测试仍然通过。
	 */
	seenTiers := make(map[string]struct{})
	seenTierCounts := make(map[string]int)
	seenCategories := make(map[string]struct{})
	seenItemScenes := make(map[string]struct{})
	petTaggedItems := 0
	highestSpendPrice := int64(0)
	lowestSpendPrice := int64(0)
	changeSpendItems := 0
	for _, item := range bootstrap.Items {
		seenTiers[item.Tier] = struct{}{}
		seenTierCounts[item.Tier] += 1
		seenCategories[item.Category] = struct{}{}
		if item.SceneID != nil {
			seenItemScenes[*item.SceneID] = struct{}{}
		}
		if item.Category == "宠物" || stringSliceContains(item.Tags, "pet") {
			petTaggedItems += 1
		}
		if item.Tier == "income" {
			continue
		}
		if item.Price > 0 && (lowestSpendPrice == 0 || item.Price < lowestSpendPrice) {
			lowestSpendPrice = item.Price
		}
		if item.Price <= 50 {
			changeSpendItems += 1
		}
		if item.Price > highestSpendPrice {
			highestSpendPrice = item.Price
		}
	}

	requiredTiers := []string{"coin", "small", "daily", "premium", "large", "heavy", "shock", "income"}
	for _, tier := range requiredTiers {
		if _, exists := seenTiers[tier]; !exists {
			t.Fatalf("database bootstrap is missing item tier %q", tier)
		}
		if seenTierCounts[tier] < minimumBootstrapTierItems {
			t.Fatalf("database bootstrap item tier %q count = %d, want at least %d", tier, seenTierCounts[tier], minimumBootstrapTierItems)
		}
	}
	if lowestSpendPrice > 1 {
		t.Fatalf("lowest spend item price = %d, want a 1-yuan spend card for the change stage", lowestSpendPrice)
	}
	if changeSpendItems < minimumChangeSpendItems {
		t.Fatalf("change spend items = %d, want at least %d spend cards at or below 50", changeSpendItems, minimumChangeSpendItems)
	}
	if highestSpendPrice < 300_000 {
		t.Fatalf("highest spend item price = %d, want shock-level spend cards near the configured cap", highestSpendPrice)
	}
	if len(seenCategories) < minimumBootstrapItemCategories {
		t.Fatalf("item categories = %d, want at least %d", len(seenCategories), minimumBootstrapItemCategories)
	}
	if len(seenItemScenes) < minimumBootstrapItemScenes {
		t.Fatalf("item scene links = %d, want at least %d", len(seenItemScenes), minimumBootstrapItemScenes)
	}

	petTaggedScenes := 0
	for _, scene := range bootstrap.Scenes {
		if stringSliceContains(scene.ItemTags, "pet") || stringSliceContains(scene.EventTags, "pet") {
			petTaggedScenes += 1
		}
	}
	petTaggedEvents := 0
	for _, event := range bootstrap.Events {
		if stringSliceContains(event.Tags, "pet") {
			petTaggedEvents += 1
		}
	}
	petTaggedStatuses := 0
	for _, effect := range bootstrap.Statuses {
		if stringSliceContains(effect.Tags, "pet") {
			petTaggedStatuses += 1
		}
	}
	if petTaggedItems < 4 {
		t.Fatalf("pet tagged items = %d, want at least 4 cards for the development-plan pet pack", petTaggedItems)
	}
	if petTaggedScenes < 1 {
		t.Fatalf("pet tagged scenes = %d, want at least 1 scene for the development-plan pet pack", petTaggedScenes)
	}
	if petTaggedEvents < 2 {
		t.Fatalf("pet tagged events = %d, want pet purchases to have dedicated event feedback", petTaggedEvents)
	}
	if petTaggedStatuses < 1 {
		t.Fatalf("pet tagged statuses = %d, want pet purchases to enter at least one status context", petTaggedStatuses)
	}
}

func TestDatabaseBootstrapContentMatchesFrontendContract(t *testing.T) {
	state := openIntegrationDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	bootstrap, err := state.loadBootstrapFromDatabase(ctx)
	if err != nil {
		t.Fatalf("load bootstrap from database: %v", err)
	}

	if bootstrap.Config.DefaultMode != defaultMode {
		t.Fatalf("default mode = %q, want %q", bootstrap.Config.DefaultMode, defaultMode)
	}
	if bootstrap.Config.InitialBalance <= 0 || bootstrap.Config.RoundLimitMs <= 0 {
		t.Fatalf("invalid bootstrap config: initialBalance=%d roundLimitMs=%d", bootstrap.Config.InitialBalance, bootstrap.Config.RoundLimitMs)
	}

	tuning := bootstrap.Config.BalanceTuning
	if tuning.StageCount <= 0 || tuning.StageDurationMs <= 0 || tuning.TargetClearMs <= 0 || tuning.HandRefreshMs <= 0 {
		t.Fatalf("invalid balance tuning timing: %+v", tuning)
	}
	if tuning.SelectionSettleMs <= 0 || tuning.InterestStartDelayMs <= 0 || tuning.InterestIntervalMs <= 0 {
		t.Fatalf("invalid balance tuning delays: %+v", tuning)
	}
	if tuning.HighPriceThreshold <= 0 || tuning.ClearCartPickCount <= 0 {
		t.Fatalf("invalid balance tuning thresholds: %+v", tuning)
	}
	if len(tuning.InterestBands) == 0 {
		t.Fatalf("balance tuning must include interest bands")
	}
	for _, band := range tuning.InterestBands {
		if band.MinBalance < 0 || band.Rate <= 0 {
			t.Fatalf("invalid interest band: %+v", band)
		}
	}

	hasSingleMultiplier := false
	for _, rule := range tuning.MultiplierRules {
		if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Label) == "" {
			t.Fatalf("multiplier rule has empty identity: %+v", rule)
		}
		if rule.Multiplier <= 0 || rule.MinBalance < 0 || rule.MaxUnitPrice <= 0 || rule.MaxTotalPrice <= 0 || rule.Weight <= 0 {
			t.Fatalf("invalid multiplier rule: %+v", rule)
		}
		if rule.Multiplier == 1 {
			hasSingleMultiplier = true
		}
	}
	if !hasSingleMultiplier {
		t.Fatalf("balance tuning must include a x1 multiplier fallback")
	}

	allowedTiers := map[string]struct{}{
		"coin": {}, "small": {}, "daily": {}, "premium": {}, "large": {}, "heavy": {}, "shock": {}, "income": {},
	}
	allowedRarities := map[string]struct{}{"common": {}, "rare": {}, "wild": {}}
	allowedEndingEffects := map[string]struct{}{"none": {}, "zero": {}}
	allowedAudioMoods := map[string]struct{}{"menu": {}, "rush": {}, "danger": {}, "settlement": {}}

	sceneIDs := make(map[string]struct{}, len(bootstrap.Scenes))
	commonSceneCount := 0
	specialSceneCount := 0
	availableEventTags := make(map[string]struct{})
	expectedSceneDurationSec := bootstrap.Config.BalanceTuning.StageDurationMs / 1000
	for _, scene := range bootstrap.Scenes {
		if strings.TrimSpace(scene.ID) == "" || strings.TrimSpace(scene.Name) == "" {
			t.Fatalf("scene has empty identity: %+v", scene)
		}
		if _, exists := allowedRarities[scene.Rarity]; !exists {
			t.Fatalf("scene %q has unsupported rarity %q", scene.ID, scene.Rarity)
		}
		if scene.RiskLevel < 1 || scene.RiskLevel > 5 || scene.MinBalance < 0 || scene.DurationSec <= 0 {
			t.Fatalf("scene %q has invalid pacing fields: %+v", scene.ID, scene)
		}
		if scene.DurationSec != expectedSceneDurationSec {
			t.Fatalf("scene %q durationSec = %d, want %d", scene.ID, scene.DurationSec, expectedSceneDurationSec)
		}
		if !stringSliceContains(scene.Modes, defaultMode) {
			t.Fatalf("scene %q modes %v do not include %q", scene.ID, scene.Modes, defaultMode)
		}
		if scene.Rarity == "common" {
			commonSceneCount += 1
		} else {
			specialSceneCount += 1
		}
		for _, tag := range scene.ItemTags {
			availableEventTags[tag] = struct{}{}
		}
		for _, tag := range scene.EventTags {
			availableEventTags[tag] = struct{}{}
		}
		sceneIDs[scene.ID] = struct{}{}
	}
	if commonSceneCount < minimumBootstrapCommonScenes {
		t.Fatalf("database bootstrap common scenes = %d, want at least %d", commonSceneCount, minimumBootstrapCommonScenes)
	}
	if specialSceneCount < minimumBootstrapSpecialScenes {
		t.Fatalf("database bootstrap special scenes = %d, want at least %d", specialSceneCount, minimumBootstrapSpecialScenes)
	}

	for _, item := range bootstrap.Items {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Category) == "" {
			t.Fatalf("item has empty identity: %+v", item)
		}
		if _, exists := allowedTiers[item.Tier]; !exists {
			t.Fatalf("item %q has unsupported tier %q", item.ID, item.Tier)
		}
		if item.Price <= 0 || item.Weight <= 0 || item.MinBalance < 0 {
			t.Fatalf("item %q has invalid money/weight fields: %+v", item.ID, item)
		}
		if item.MaxBuy != nil && *item.MaxBuy <= 0 {
			t.Fatalf("item %q has invalid maxBuy %d", item.ID, *item.MaxBuy)
		}
		if !stringSliceContains(item.Modes, defaultMode) {
			t.Fatalf("item %q modes %v do not include %q", item.ID, item.Modes, defaultMode)
		}
		if item.SceneID != nil {
			if _, exists := sceneIDs[*item.SceneID]; !exists {
				t.Fatalf("item %q references scene %q that is not present in the active bootstrap scene list", item.ID, *item.SceneID)
			}
		}
		if len(item.Tags) == 0 {
			t.Fatalf("item %q must include at least one tag for scene/event matching", item.ID)
		}
		for _, tag := range item.Tags {
			availableEventTags[tag] = struct{}{}
		}
	}

	for _, event := range bootstrap.Events {
		if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Title) == "" || strings.TrimSpace(event.Description) == "" {
			t.Fatalf("event has empty identity: %+v", event)
		}
		if event.Probability <= 0 || event.Probability > maximumChaosEventProbability || event.CooldownSec < 0 {
			t.Fatalf("event %q has invalid probability/cooldown: %+v", event.ID, event)
		}
		if event.Delta != nil {
			maximumChaosEventDelta := bootstrap.Config.InitialBalance / maximumChaosEventDeltaDivisor
			if maximumChaosEventDelta < 1 {
				maximumChaosEventDelta = 1
			}
			if *event.Delta < -maximumChaosEventDelta || *event.Delta > maximumChaosEventDelta {
				t.Fatalf("event %q delta %d outside allowed range", event.ID, *event.Delta)
			}
		}
		if !stringSliceContains(event.Modes, defaultMode) {
			t.Fatalf("event %q modes %v do not include %q", event.ID, event.Modes, defaultMode)
		}
		if len(event.Tags) == 0 {
			t.Fatalf("event %q must include at least one tag for purchase matching", event.ID)
		}
		hasMatchingTag := false
		for _, tag := range event.Tags {
			if _, exists := availableEventTags[tag]; exists {
				hasMatchingTag = true
				break
			}
		}
		if !hasMatchingTag {
			t.Fatalf("event %q tags %v do not match any item or scene tags", event.ID, event.Tags)
		}
	}

	for _, ending := range bootstrap.Endings {
		if strings.TrimSpace(ending.ID) == "" || strings.TrimSpace(ending.Title) == "" || strings.TrimSpace(ending.Description) == "" {
			t.Fatalf("ending has empty identity: %+v", ending)
		}
		if ending.Probability < 0 || ending.Probability > 1 || ending.MinElapsedMs < 0 || ending.MinRiskLevel < 1 || ending.MinRiskLevel > 5 {
			t.Fatalf("ending %q has invalid trigger fields: %+v", ending.ID, ending)
		}
		if ending.Probability > maxTerminalEventProbability {
			t.Fatalf("ending %q probability %.4f is above allowed terminal range", ending.ID, ending.Probability)
		}
		if _, exists := allowedEndingEffects[ending.BalanceEffect]; !exists {
			t.Fatalf("ending %q has unsupported balance effect %q", ending.ID, ending.BalanceEffect)
		}
		if !stringSliceContains(ending.Modes, defaultMode) {
			t.Fatalf("ending %q modes %v do not include %q", ending.ID, ending.Modes, defaultMode)
		}
	}

	for _, effect := range bootstrap.Statuses {
		if strings.TrimSpace(effect.ID) == "" || strings.TrimSpace(effect.Name) == "" || strings.TrimSpace(effect.Description) == "" {
			t.Fatalf("status effect has empty identity: %+v", effect)
		}
		if effect.DurationSec <= 0 || effect.ItemRefreshMultiplier <= 0 || effect.HighPriceMultiplier <= 0 || effect.EventMultiplier <= 0 {
			t.Fatalf("status effect %q has invalid multiplier fields: %+v", effect.ID, effect)
		}
		if effect.DurationSec < minimumStatusDurationSec || effect.DurationSec > maximumStatusDurationSec {
			t.Fatalf("status effect %q durationSec %d outside allowed range", effect.ID, effect.DurationSec)
		}
		if effect.ItemRefreshMultiplier < 0.5 || effect.ItemRefreshMultiplier > 1.8 {
			t.Fatalf("status effect %q itemRefreshMultiplier %.2f outside allowed range", effect.ID, effect.ItemRefreshMultiplier)
		}
		if effect.HighPriceMultiplier < 0.5 || effect.HighPriceMultiplier > 2 {
			t.Fatalf("status effect %q highPriceMultiplier %.2f outside allowed range", effect.ID, effect.HighPriceMultiplier)
		}
		if effect.EventMultiplier < 0.5 || effect.EventMultiplier > 1.8 {
			t.Fatalf("status effect %q eventMultiplier %.2f outside allowed range", effect.ID, effect.EventMultiplier)
		}
		if len(effect.Tags) == 0 {
			t.Fatalf("status effect %q has no matching tags", effect.ID)
		}
	}

	for _, track := range bootstrap.AudioTracks {
		if strings.TrimSpace(track.ID) == "" || strings.TrimSpace(track.Title) == "" {
			t.Fatalf("audio track has empty identity: %+v", track)
		}
		if _, exists := allowedAudioMoods[track.Mood]; !exists {
			t.Fatalf("audio track %q has unsupported mood %q", track.ID, track.Mood)
		}
	}
}

func TestDatabaseSubmitRunRejectsDuplicateUsername(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成重复"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	run := integrationRun(username, "balance_zero", 2_000, 80_000)
	if _, err := state.submitRunToDatabase(ctx, run); err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	if _, err := state.submitRunToDatabase(ctx, run); err == nil {
		t.Fatalf("expected duplicate submit to be rejected")
	} else if !errors.Is(err, errDuplicateRun) {
		t.Fatalf("unexpected duplicate error: %v", err)
	}
}

func TestDatabaseSubmitRunRejectsInvalidPayload(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成非法"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	run := integrationRun(integrationUsername(prefix), "timeout", defaultRoundLimitMs, 80_000)
	run.FinalBalance = 0
	run.TotalSpent = defaultInitialBalance + run.TotalIncome

	/*
	 * HTTP handler 已经会在进入数据库前调用 validateRun，但 submitRunToDatabase 也可能被
	 * 集成测试、未来后台任务或内部工具直接调用。这里锁住数据库写入函数自己的边界：即使
	 * 绕过 HTTP 层，`timeout` 但余额为 0 这种矛盾成绩也不能写入 runs 表。
	 */
	if _, err := state.submitRunToDatabase(ctx, run); err == nil {
		t.Fatalf("expected direct database submit with invalid payload to be rejected")
	}
}

func TestDatabaseRunsTableRejectsInvalidDirectRows(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成表约束"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	tests := []struct {
		name   string
		mutate func(*runSubmission)
	}{
		{
			name: "duration too short",
			mutate: func(run *runSubmission) {
				run.DurationMs = minAcceptedDurationMs - 1
			},
		},
		{
			name: "duration after hard round limit",
			mutate: func(run *runSubmission) {
				run.DurationMs = defaultRoundLimitMs + 1
			},
		},
		{
			name: "money fields do not balance",
			mutate: func(run *runSubmission) {
				run.FinalBalance = 123
			},
		},
		{
			name: "max single spend exceeds total spent",
			mutate: func(run *runSubmission) {
				run.TotalSpent = 10
				run.MaxSingleSpend = 11
				run.FinalBalance = defaultInitialBalance - run.TotalSpent
			},
		},
		{
			name: "balance zero with remaining balance",
			mutate: func(run *runSubmission) {
				run.FinalBalance = 100
				run.TotalSpent = defaultInitialBalance - run.FinalBalance
			},
		},
		{
			name: "timeout with zero balance",
			mutate: func(run *runSubmission) {
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				run.FinalBalance = 0
				run.TotalSpent = defaultInitialBalance
			},
		},
		{
			name: "manual with zero balance",
			mutate: func(run *runSubmission) {
				run.EndedBy = "manual"
				run.FinalBalance = 0
				run.TotalSpent = defaultInitialBalance
			},
		},
		{
			name: "timeout before hard round limit",
			mutate: func(run *runSubmission) {
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs - 1
				run.FinalBalance = 10_000
				run.TotalSpent = defaultInitialBalance + run.TotalIncome - run.FinalBalance
			},
		},
		{
			name: "blank chaos seed",
			mutate: func(run *runSubmission) {
				run.ChaosSeed = " "
			},
		},
		{
			name: "blank content version",
			mutate: func(run *runSubmission) {
				run.ContentVersion = " "
			},
		},
		{
			name: "malformed content version",
			mutate: func(run *runSubmission) {
				run.ContentVersion = "manual-content-version"
			},
		},
		{
			name: "terminal event without id",
			mutate: func(run *runSubmission) {
				run.EndedBy = "terminal_event"
				run.FinalBalance = 0
				run.TotalSpent = defaultInitialBalance
				run.EndingTitle = "测试终局"
				run.EndingDetail = "测试终局详情"
			},
		},
		{
			name: "terminal event without title",
			mutate: func(run *runSubmission) {
				run.EndedBy = "terminal_event"
				run.FinalBalance = 0
				run.TotalSpent = defaultInitialBalance
				run.EndingID = "test-ending"
				run.EndingTitle = ""
				run.EndingDetail = "测试终局详情"
			},
		},
		{
			name: "terminal event without detail",
			mutate: func(run *runSubmission) {
				run.EndedBy = "terminal_event"
				run.FinalBalance = 0
				run.TotalSpent = defaultInitialBalance
				run.EndingID = "test-ending"
				run.EndingTitle = "测试终局"
				run.EndingDetail = ""
			},
		},
		{
			name: "non-terminal with terminal event fields",
			mutate: func(run *runSubmission) {
				run.EndingID = "test-ending"
				run.EndingTitle = "不该出现的终局"
				run.EndingDetail = "普通清空成绩不应该夹带终局文案"
			},
		},
		{
			name: "money field exceeds upper bound",
			mutate: func(run *runSubmission) {
				run.EndedBy = "timeout"
				run.DurationMs = defaultRoundLimitMs
				run.FinalBalance = maxAcceptedRunMoney + 1
				run.TotalSpent = 0
				run.TotalIncome = run.FinalBalance - defaultInitialBalance
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			username := integrationUsername(prefix)
			run := integrationRun(username, "balance_zero", 2_000, 80_000)
			test.mutate(&run)

			if _, err := state.db.ExecContext(ctx, `INSERT INTO usernames (username) VALUES ($1)`, username); err != nil {
				t.Fatalf("insert username: %v", err)
			}

			/*
			 * 这条 INSERT 故意绕过 submitRunToDatabase，直接打到 runs 表。Go API 和内部
			 * 写库函数都会先做 validateRun，但数据库仍然应该兜住最后一道边界，防止维护
			 * 脚本、后台任务或手工 SQL 写入明显矛盾的排行榜成绩。
			 */
			_, err := state.db.ExecContext(ctx, `
				INSERT INTO runs (
					username, duration_ms, max_single_spend, final_balance, total_spent, total_income, ended_by, chaos_seed,
					content_version, ending_id, ending_title, ending_detail
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''))
			`, run.Username, run.DurationMs, run.MaxSingleSpend, run.FinalBalance, run.TotalSpent, run.TotalIncome, run.EndedBy, run.ChaosSeed, run.ContentVersion, run.EndingID, run.EndingTitle, run.EndingDetail)
			if err == nil {
				t.Fatalf("expected direct runs insert for %s to be rejected", test.name)
			}
		})
	}
}

func TestDatabaseUsernamesTableRejectsInvalidDirectRows(t *testing.T) {
	state := openIntegrationDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	tests := []string{
		"短",
		"带/斜杠",
		"带<尖括号>",
		"前后空格 ",
		"多个  空格",
		"这个用户名长度已经明显超过十六个字符限制",
	}

	for _, username := range tests {
		t.Run(username, func(t *testing.T) {
			/*
			 * Go API 会先规范和校验用户名，但数据库仍然需要最后一道约束。这样后续维护脚本、
			 * 管理后台或手工 SQL 即使绕过 HTTP handler，也不能把和排行榜规则不一致的名字
			 * 写进 usernames 表，再被 runs 表外键引用成公开成绩。
			 */
			if _, err := state.db.ExecContext(ctx, `INSERT INTO usernames (username) VALUES ($1)`, username); err == nil {
				t.Fatalf("expected direct username insert for %q to be rejected", username)
			}
		})
	}
}

func TestDatabaseUsernamesTableRejectsLeaseWithoutToken(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成空租约"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * reserved_until 表示“这个用户名现在有一段租约”。这段租约只有和 reservation_token
	 * 一起存在才有意义，因为浏览器后续开局和提交成绩时要带回同一个 token 证明自己是
	 * 当初预约的人。如果数据库允许 reserved_until 有值但 token 为空，Go 续租逻辑会看到
	 * 一个被占用的名字，却没有任何客户端能拿出匹配 token，最终造成一段无法续租的假占用。
	 */
	tests := []struct {
		name  string
		token string
	}{
		{name: "blank token", token: ""},
		{name: "space token", token: " "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			username := integrationUsername(prefix)
			if _, err := state.db.ExecContext(ctx, `
				INSERT INTO usernames (username, reservation_token, reserved_until)
				VALUES ($1, $2, now() + interval '30 minutes')
			`, username, test.token); err == nil {
				t.Fatalf("expected reserved username without usable token to be rejected")
			}
		})
	}
}

func TestDatabaseSubmitRunReservesUsernameForDirectSubmit(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成直提"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	if _, err := state.submitRunToDatabase(ctx, integrationRun(username, "balance_zero", 2_000, 80_000)); err != nil {
		t.Fatalf("direct submit failed: %v", err)
	}

	reserved, _, err := state.reserveUsernameInDatabase(ctx, username, "")
	if err != nil {
		t.Fatalf("reserve after direct submit failed: %v", err)
	}
	if reserved {
		t.Fatalf("reserve after direct submit returned true, want false")
	}
}

func TestDatabaseReserveUsernameRenewsAndReleasesExpiredLease(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成预约"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	reserved, token, err := state.reserveUsernameInDatabase(ctx, username, "")
	if err != nil {
		t.Fatalf("first reserve failed: %v", err)
	}
	if !reserved || token == "" {
		t.Fatalf("first reserve = (%v, %q), want reserved with token", reserved, token)
	}

	reserved, _, err = state.reserveUsernameInDatabase(ctx, username, "")
	if err != nil {
		t.Fatalf("second reserve without token failed: %v", err)
	}
	if reserved {
		t.Fatalf("active reservation without matching token should stay blocked")
	}

	reserved, renewedToken, err := state.reserveUsernameInDatabase(ctx, username, token)
	if err != nil {
		t.Fatalf("reserve with matching token failed: %v", err)
	}
	if !reserved || renewedToken != token {
		t.Fatalf("renew reserve = (%v, %q), want original token %q", reserved, renewedToken, token)
	}

	if _, err := state.db.ExecContext(ctx, `UPDATE usernames SET reserved_until = now() - interval '1 second' WHERE username = $1`, username); err != nil {
		t.Fatalf("expire reservation: %v", err)
	}
	reserved, freshToken, err := state.reserveUsernameInDatabase(ctx, username, "")
	if err != nil {
		t.Fatalf("reserve expired username failed: %v", err)
	}
	if !reserved || freshToken == "" {
		t.Fatalf("expired reserve = (%v, %q), want fresh reservation token", reserved, freshToken)
	}

	legacyUsername := integrationUsername(prefix + "旧")
	if _, err := state.db.ExecContext(ctx, `INSERT INTO usernames (username) VALUES ($1)`, legacyUsername); err != nil {
		t.Fatalf("insert legacy username reservation: %v", err)
	}
	reserved, legacyToken, err := state.reserveUsernameInDatabase(ctx, legacyUsername, "")
	if err != nil {
		t.Fatalf("reserve legacy username failed: %v", err)
	}
	if !reserved || legacyToken == "" {
		t.Fatalf("legacy reserve = (%v, %q), want fresh reservation token", reserved, legacyToken)
	}
}

func TestDatabaseSubmitRunRequiresMatchingReservationToken(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成提交预约"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	reserved, token, err := state.reserveUsernameInDatabase(ctx, username, "")
	if err != nil {
		t.Fatalf("reserve username: %v", err)
	}
	if !reserved || token == "" {
		t.Fatalf("reserve = (%v, %q), want token", reserved, token)
	}

	if _, err := state.submitRunToDatabase(ctx, integrationRun(username, "balance_zero", 2_000, 80_000)); !errors.Is(err, errUsernameReserved) {
		t.Fatalf("submit without reservation token error = %v, want errUsernameReserved", err)
	}

	run := integrationRun(username, "balance_zero", 2_000, 80_000)
	run.ReservationToken = token
	if _, err := state.submitRunToDatabase(ctx, run); err != nil {
		t.Fatalf("submit with reservation token failed: %v", err)
	}
}

func TestDatabaseSubmitRunReturnsRankedEntry(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成返回"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	run := integrationRun(username, "balance_zero", 2_000, 80_000)
	entry, err := state.submitRunToDatabase(ctx, run)
	if err != nil {
		t.Fatalf("submit run failed: %v", err)
	}

	/*
	 * submitRunToDatabase 的返回值会直接给前端结算页显示。如果这里未来改成用请求体
	 * 临时拼一个 entry，而不是读取数据库排序后的 entry，前端就可能显示和排行榜不同的
	 * 名次。这个断言把返回值和数据库回查绑定在一起。
	 */
	ranked, err := state.leaderboardEntryForUsername(ctx, username)
	if err != nil {
		t.Fatalf("rank submitted entry: %v", err)
	}
	if entry != ranked {
		t.Fatalf("returned entry = %+v, want database ranked entry %+v", entry, ranked)
	}
}

func TestDatabaseSubmitRunStoresContentVersion(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成内容版本"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	run := integrationRun(username, "balance_zero", 2_000, 80_000)
	run.ContentVersion = integrationContentVersion()
	if _, err := state.submitRunToDatabase(ctx, run); err != nil {
		t.Fatalf("submit run failed: %v", err)
	}

	/*
	 * 排行榜接口暂时不需要把 content_version 返回给玩家，但数据库必须保存它。这个断言
	 * 保护的是后续调卡片、调金额算法时的排查能力：看到一条旧成绩时，至少能知道它来自
	 * 哪一版内容包，而不是只剩一个无法复现的成绩数字。
	 */
	var got string
	if err := state.db.QueryRowContext(ctx, `
		SELECT content_version
		FROM runs
		WHERE username = $1
	`, username).Scan(&got); err != nil {
		t.Fatalf("query stored content version: %v", err)
	}
	if got != run.ContentVersion {
		t.Fatalf("stored content version = %q, want %q", got, run.ContentVersion)
	}
}

func TestDatabaseSubmitRunNormalizesMalformedContentVersion(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成版本降级"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	run := integrationRun(username, "balance_zero", 2_000, 80_000)
	run.ContentVersion = "manual-content-version"
	entry, err := state.submitRunToDatabase(ctx, run)
	if err != nil {
		t.Fatalf("submit run with malformed content version failed: %v", err)
	}
	if entry.Rank <= 0 {
		t.Fatalf("entry rank = %d, want a ranked response even after content-version downgrade", entry.Rank)
	}

	var got string
	if err := state.db.QueryRowContext(ctx, `
		SELECT content_version
		FROM runs
		WHERE username = $1
	`, username).Scan(&got); err != nil {
		t.Fatalf("query normalized content version: %v", err)
	}
	if got != unknownContentVersion {
		t.Fatalf("stored content version = %q, want %q for malformed client input", got, unknownContentVersion)
	}
}

func TestInitializeDatabaseNormalizesLegacyContentVersions(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成旧版本"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	username := integrationUsername(prefix)
	run := integrationRun(username, "balance_zero", 2_000, 80_000)

	/*
	 * 这个测试模拟的是已经存在的旧数据库：当时 runs.content_version 还没有被限制成
	 * `sha256:` 指纹，所以可能已经保存了人工字符串。先临时移除约束并直接插入旧值，
	 * 再调用 initializeDatabase，才能证明启动时的 schema 初始化会迁移历史数据，而不是
	 * 只保护之后的新写入。
	 */
	if _, err := state.db.ExecContext(ctx, `ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_content_version_not_blank_check`); err != nil {
		t.Fatalf("drop content version constraint: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cleanupCancel()
		if err := initializeDatabase(cleanupCtx, state.db); err != nil {
			t.Fatalf("restore schema after legacy content-version test: %v", err)
		}
	})

	if _, err := state.db.ExecContext(ctx, `INSERT INTO usernames (username) VALUES ($1)`, username); err != nil {
		t.Fatalf("insert legacy username: %v", err)
	}
	if _, err := state.db.ExecContext(ctx, `
		INSERT INTO runs (
			username, duration_ms, max_single_spend, final_balance, total_spent, total_income, ended_by, chaos_seed,
			content_version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, run.Username, run.DurationMs, run.MaxSingleSpend, run.FinalBalance, run.TotalSpent, run.TotalIncome, run.EndedBy, run.ChaosSeed, "manual-content-version"); err != nil {
		t.Fatalf("insert legacy run with malformed content version: %v", err)
	}

	if err := initializeDatabase(ctx, state.db); err != nil {
		t.Fatalf("initialize database should normalize legacy content version: %v", err)
	}

	var got string
	if err := state.db.QueryRowContext(ctx, `
		SELECT content_version
		FROM runs
		WHERE username = $1
	`, username).Scan(&got); err != nil {
		t.Fatalf("query normalized legacy content version: %v", err)
	}
	if got != unknownContentVersion {
		t.Fatalf("legacy content version = %q, want %q", got, unknownContentVersion)
	}
}

func TestDatabaseLeaderboardFiltersByContentVersion(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成版本榜"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	oldVersion := integrationContentVersion()
	newVersion := integrationContentVersion()

	oldUsername := integrationUsername(prefix + "旧")
	oldRun := integrationRun(oldUsername, "balance_zero", 2_000, 220_000)
	oldRun.ContentVersion = oldVersion
	if _, err := state.submitRunToDatabase(ctx, oldRun); err != nil {
		t.Fatalf("submit old-version run failed: %v", err)
	}

	slowUsername := integrationUsername(prefix + "慢")
	slowRun := integrationRun(slowUsername, "balance_zero", 3_000, 80_000)
	slowRun.ContentVersion = newVersion
	slowEntry, err := state.submitRunToDatabase(ctx, slowRun)
	if err != nil {
		t.Fatalf("submit first new-version run failed: %v", err)
	}
	if slowEntry.Rank != 1 {
		t.Fatalf("first filtered content-version rank = %d, want 1", slowEntry.Rank)
	}

	fastUsername := integrationUsername(prefix + "快")
	fastRun := integrationRun(fastUsername, "balance_zero", 2_500, 70_000)
	fastRun.ContentVersion = newVersion
	fastEntry, err := state.submitRunToDatabase(ctx, fastRun)
	if err != nil {
		t.Fatalf("submit second new-version run failed: %v", err)
	}
	if fastEntry.Rank != 1 {
		t.Fatalf("faster filtered content-version rank = %d, want 1", fastEntry.Rank)
	}

	/*
	 * content_version 是排行榜可比性的边界。旧 seed 或旧金额算法跑出来的成绩可以保留在
	 * 数据库里，但当前内容包的公开榜应该只拿同一版内容来排。这个测试同时锁住提交响应
	 * 和榜单查询：结算页拿到的名次，必须和右侧按当前内容版本刷新出来的名次一致。
	 */
	entries, err := state.leaderboardFromDatabase(ctx, leaderboardLimit, newVersion)
	if err != nil {
		t.Fatalf("load filtered leaderboard: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("filtered leaderboard length = %d, want 2 entries for new content version", len(entries))
	}
	if entries[0].Username != fastUsername || entries[0].Rank != 1 {
		t.Fatalf("filtered first entry = %+v, want %q at rank 1", entries[0], fastUsername)
	}
	if entries[1].Username != slowUsername || entries[1].Rank != 2 {
		t.Fatalf("filtered second entry = %+v, want %q at rank 2", entries[1], slowUsername)
	}
	for _, entry := range entries {
		if entry.Username == oldUsername {
			t.Fatalf("filtered leaderboard leaked old content-version entry: %+v", entry)
		}
	}
}

func TestDatabaseConcurrentDuplicateSubmitAllowsOneWinner(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成并发"
	cleanupIntegrationUsers(t, state, prefix)

	username := integrationUsername(prefix)
	run := integrationRun(username, "balance_zero", 2_000, 80_000)
	var waitGroup sync.WaitGroup
	errs := make(chan error, 2)

	for worker := 0; worker < 2; worker += 1 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
			defer cancel()
			_, err := state.submitRunToDatabase(ctx, run)
			errs <- err
		}()
	}

	waitGroup.Wait()
	close(errs)

	successCount := 0
	duplicateCount := 0
	for err := range errs {
		if err == nil {
			successCount += 1
			continue
		}
		if errors.Is(err, errDuplicateRun) {
			duplicateCount += 1
			continue
		}
		t.Fatalf("unexpected concurrent submit error: %v", err)
	}

	if successCount != 1 || duplicateCount != 1 {
		t.Fatalf("successCount=%d duplicateCount=%d, want 1 and 1", successCount, duplicateCount)
	}
}

func TestDatabaseLeaderboardRankingRules(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成排序"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	clearedHigh := integrationUsername(prefix + "高")
	clearedLow := integrationUsername(prefix + "低")
	terminalZero := integrationUsername(prefix + "终")
	timeoutFast := integrationUsername(prefix + "未")
	terminalRun := integrationRun(terminalZero, "terminal_event", 3_000, 90_000)
	terminalRun.FinalBalance = 0
	terminalRun.TotalSpent = defaultInitialBalance + terminalRun.TotalIncome
	terminalRun.EndingID = "test-ending-zero"
	terminalRun.EndingTitle = "测试终局清零"
	terminalRun.EndingDetail = "测试终局清零详情"
	runs := []runSubmission{
		integrationRun(timeoutFast, "timeout", defaultRoundLimitMs, 999_000),
		integrationRun(clearedLow, "balance_zero", 2_000, 80_000),
		integrationRun(clearedHigh, "balance_zero", 2_000, 180_000),
		terminalRun,
	}

	for _, run := range runs {
		if _, err := state.submitRunToDatabase(ctx, run); err != nil {
			t.Fatalf("submit %s failed: %v", run.Username, err)
		}
	}

	highEntry, err := state.leaderboardEntryForUsername(ctx, clearedHigh)
	if err != nil {
		t.Fatalf("rank high spend entry: %v", err)
	}
	lowEntry, err := state.leaderboardEntryForUsername(ctx, clearedLow)
	if err != nil {
		t.Fatalf("rank low spend entry: %v", err)
	}
	timeoutEntry, err := state.leaderboardEntryForUsername(ctx, timeoutFast)
	if err != nil {
		t.Fatalf("rank timeout entry: %v", err)
	}
	terminalEntry, err := state.leaderboardEntryForUsername(ctx, terminalZero)
	if err != nil {
		t.Fatalf("rank terminal zero entry: %v", err)
	}

	if !(highEntry.Rank < lowEntry.Rank) {
		t.Fatalf("same-duration balance_zero max spend order failed: high=%d low=%d", highEntry.Rank, lowEntry.Rank)
	}
	if !(lowEntry.Rank < terminalEntry.Rank && terminalEntry.Rank < timeoutEntry.Rank) {
		t.Fatalf("final_balance zero ordering failed: low=%d terminal=%d timeout=%d", lowEntry.Rank, terminalEntry.Rank, timeoutEntry.Rank)
	}
}

func TestDatabaseLeaderboardUsesIDAsFinalTieBreaker(t *testing.T) {
	state := openIntegrationDatabase(t)
	prefix := "集成同分"
	cleanupIntegrationUsers(t, state, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	firstUsername := integrationUsername(prefix + "先")
	secondUsername := integrationUsername(prefix + "后")

	for _, username := range []string{firstUsername, secondUsername} {
		run := integrationRun(username, "balance_zero", 6_000, 90_000)
		if _, err := state.submitRunToDatabase(ctx, run); err != nil {
			t.Fatalf("submit tied run %s failed: %v", username, err)
		}
	}

	/*
	 * PostgreSQL 的 created_at 默认值精度已经很高，但并发写入或人工维护数据时仍可能出现
	 * 两条成绩在“余额是否归零、用时、最大单笔、创建时间”上完全相同。排行榜不能把这种
	 * 情况交给数据库的未定义行顺序处理，所以查询和索引都要把自增 id 作为最后排序字段。
	 * 这里把两条测试成绩的 created_at 强制改成同一刻，验证更早插入、id 更小的成绩稳定排前。
	 */
	if _, err := state.db.ExecContext(ctx, `
		UPDATE runs
		SET created_at = TIMESTAMPTZ '2026-07-05 00:00:00+00'
		WHERE username IN ($1, $2)
	`, firstUsername, secondUsername); err != nil {
		t.Fatalf("force tied created_at: %v", err)
	}

	firstEntry, err := state.leaderboardEntryForUsername(ctx, firstUsername)
	if err != nil {
		t.Fatalf("rank first tied entry: %v", err)
	}
	secondEntry, err := state.leaderboardEntryForUsername(ctx, secondUsername)
	if err != nil {
		t.Fatalf("rank second tied entry: %v", err)
	}
	if !(firstEntry.Rank < secondEntry.Rank) {
		t.Fatalf("id tie-breaker order failed: first rank=%d second rank=%d", firstEntry.Rank, secondEntry.Rank)
	}
}
