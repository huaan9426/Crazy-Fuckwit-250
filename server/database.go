package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	databaseRequestTimeout         = 4 * time.Second
	databaseStartupTimeout         = 8 * time.Second
	minimumBootstrapItems          = 300
	minimumBootstrapScenes         = 20
	minimumBootstrapEvents         = 80
	minimumBootstrapStatuses       = 12
	minimumBootstrapEndings        = 6
	minimumBootstrapAudioTracks    = 3
	minimumBootstrapCommonScenes   = 6
	minimumBootstrapSpecialScenes  = 10
	minimumBootstrapItemCategories = 16
	minimumBootstrapItemScenes     = 18
	minimumBootstrapTierItems      = 6
	minimumChangeSpendItems        = 5
	minimumStatusDurationSec       = 8
	maximumStatusDurationSec       = 45
	maximumChaosEventProbability   = 0.25
	maximumChaosEventDeltaDivisor  = 25
	maxTerminalEventProbability    = 0.005
)

//go:embed db/schema.sql db/seed.sql
var databaseSQLFiles embed.FS

// openConfiguredDatabase 只在配置了 DATABASE_URL 时启用 PostgreSQL。这样本地只想跑前端
// 或者暂时没有启动数据库时，旧的内存/静态兜底仍然能工作；真正部署时，只要提供
// DATABASE_URL，用户名、成绩、排行榜、终局事件和内容包就会走数据库。
func openConfiguredDatabase() (*sql.DB, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, nil
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), databaseStartupTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	if err := initializeDatabase(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func initializeDatabase(ctx context.Context, db *sql.DB) error {
	schemaSQL, err := databaseSQLFiles.ReadFile("db/schema.sql")
	if err != nil {
		return err
	}

	seedSQL, err := databaseSQLFiles.ReadFile("db/seed.sql")
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	if _, err := tx.ExecContext(ctx, string(seedSQL)); err != nil {
		return fmt.Errorf("apply seed: %w", err)
	}

	return tx.Commit()
}

func (state *appState) hasDatabase() bool {
	return state.db != nil
}

func (state *appState) loadBootstrap(ctx context.Context) (bootstrapResponse, error) {
	if state.db == nil {
		return withContentVersion(bootstrapContent()), nil
	}

	bootstrap, err := state.loadBootstrapFromDatabase(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	return bootstrap, nil
}

func (state *appState) loadBootstrapFromDatabase(ctx context.Context) (bootstrapResponse, error) {
	scenes, err := state.loadScenes(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	items, err := state.loadItems(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	events, err := state.loadEvents(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	endings, err := state.loadEndings(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	statuses, err := state.loadStatuses(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	tracks, err := state.loadAudioTracks(ctx)
	if err != nil {
		return bootstrapResponse{}, err
	}

	bootstrap := bootstrapResponse{
		Config: gameConfig{
			InitialBalance: defaultInitialBalance,
			RoundLimitMs:   defaultRoundLimitMs,
			DefaultMode:    defaultMode,
			BalanceTuning:  deriveBalanceTuning(defaultInitialBalance, defaultRoundLimitMs),
		},
		Items:       items,
		Scenes:      scenes,
		Events:      events,
		Endings:     endings,
		Statuses:    statuses,
		AudioTracks: tracks,
	}
	bootstrap = withContentVersion(bootstrap)
	if err := validateBootstrapContent(bootstrap); err != nil {
		return bootstrapResponse{}, err
	}

	return bootstrap, nil
}

func validateBootstrapContent(bootstrap bootstrapResponse) error {
	/*
	 * PostgreSQL 内容包是正式主线，不只是“有商品和场景就能开局”。事件、状态、终局事件
	 * 和音轨入口也会影响前端玩法：事件决定购买后的混沌反馈，状态影响刷新和概率，终局事件
	 * 生成特殊战报，音轨入口决定后续真实音乐资源怎么接入。如果这些核心分类被误删或全部
	 * 设为 inactive，API 返回 200 会让前端拿着残缺内容运行，问题会在游戏过程中才暴露。
	 * 这里在服务端边界先拒绝残缺内容，让数据库/seed 问题尽早变成可见错误。
	 */
	requiredMinimums := []struct {
		name string
		got  int
		want int
	}{
		{name: "items", got: len(bootstrap.Items), want: minimumBootstrapItems},
		{name: "scenes", got: len(bootstrap.Scenes), want: minimumBootstrapScenes},
		{name: "events", got: len(bootstrap.Events), want: minimumBootstrapEvents},
		{name: "endings", got: len(bootstrap.Endings), want: minimumBootstrapEndings},
		{name: "statuses", got: len(bootstrap.Statuses), want: minimumBootstrapStatuses},
		{name: "audio tracks", got: len(bootstrap.AudioTracks), want: minimumBootstrapAudioTracks},
	}
	for _, required := range requiredMinimums {
		if required.got < required.want {
			return fmt.Errorf("database content category %q count = %d, want at least %d", required.name, required.got, required.want)
		}
	}
	if err := validateBalanceTuning(bootstrap.Config); err != nil {
		return err
	}

	expectedSceneDurationSec := bootstrap.Config.BalanceTuning.StageDurationMs / 1000
	commonSceneCount := 0
	specialSceneCount := 0
	sceneIDs := make(map[string]struct{}, len(bootstrap.Scenes))
	availableEventTags := make(map[string]struct{})
	for _, scene := range bootstrap.Scenes {
		/*
		 * SQL 查询已经写了 `modes ? defaultMode`，但 PostgreSQL JSONB 的 `?` 不只会检查数组
		 * 元素，也会检查对象 key。如果内容编辑把 `["chaos-life"]` 误写成
		 * `{"chaos-life": true}`，这条记录仍会被查询选中；Go 再把这个 JSONB 解成 []string
		 * 时会得到空数组。这里再次检查 modes 数组里确实包含默认模式，保证“数据库筛选到了”
		 * 和“前端收到的 modes 字段”说的是同一件事。
		 */
		if !textSliceContains(scene.Modes, defaultMode) {
			return fmt.Errorf("database scene %q modes must include %q", scene.ID, defaultMode)
		}
		/*
		 * entryCost 和 minBalance 都是场景进入前端节奏算法的金额字段。entryCost 表示进入
		 * 某个高压场景时先扣掉的服务费或押金，minBalance 表示玩家余额至少到多少时才会
		 * 把这个场景放进可选池。负数没有真实玩法含义：负入场费会被前端当成“不收费”，
		 * 负余额门槛则会让场景永远满足条件。这里在内容包边界拒绝这类数据，让数据库作者
		 * 不能靠一个负数绕过前端本来表达的场景门槛。
		 */
		if scene.EntryCost < 0 || scene.MinBalance < 0 {
			return fmt.Errorf("database scene %q has negative money gate", scene.ID)
		}

		/*
		 * 前端的一局会把 12 个阶段分成普通场景循环和少量特殊场景插入。common 场景就是
		 * 普通循环的基础货架：它让大部分时间保持日常、小额、中额消费为主，rare/wild 场景
		 * 才负责偶尔提高风险。如果数据库只剩 rare 或 wild，内容包虽然“场景非空”，但前端
		 * 会退化成拿第一个场景当普通场景，金额算法和阶段情绪都会偏离开发计划。
		 */
		if scene.Rarity == "common" {
			commonSceneCount += 1
		} else {
			specialSceneCount += 1
		}

		/*
		 * scene.durationSec 是内容表里给场景写的持续时间说明，但真正推动阶段变化的时钟
		 * 是 balanceTuning.stageDurationMs。它们如果写成两套数字，前端仍会按阶段时钟切换，
		 * 内容编辑却会以为某个高压场景只持续 18 秒，最后形成“数据库看起来配置了，游戏里
		 * 没有按它执行”的断点。当前首版只有固定 12 阶段，所以这里要求场景声明的秒数和
		 * 后端算法推导出的阶段秒数一致；以后如果真要做变长场景，应先改前端阶段时钟，再
		 * 放开这个校验，而不是只改 seed。
		 */
		if scene.DurationSec != expectedSceneDurationSec {
			return fmt.Errorf("database scene %q durationSec = %d, want stage duration %d", scene.ID, scene.DurationSec, expectedSceneDurationSec)
		}
		sceneIDs[scene.ID] = struct{}{}
		for _, tag := range scene.ItemTags {
			availableEventTags[tag] = struct{}{}
		}
		for _, tag := range scene.EventTags {
			availableEventTags[tag] = struct{}{}
		}
	}
	if commonSceneCount < minimumBootstrapCommonScenes {
		return fmt.Errorf("database content common scenes = %d, want at least %d", commonSceneCount, minimumBootstrapCommonScenes)
	}
	if specialSceneCount < minimumBootstrapSpecialScenes {
		return fmt.Errorf("database content special scenes = %d, want at least %d", specialSceneCount, minimumBootstrapSpecialScenes)
	}

	hasPayableSpendItem := false
	hasIncomeItem := false
	seenItemCategories := make(map[string]struct{})
	seenItemSceneLinks := make(map[string]struct{})
	seenItemTiers := make(map[string]struct{})
	seenItemTierCounts := make(map[string]int)
	lowestSpendItemPrice := int64(0)
	changeSpendItemCount := 0
	highestSpendItemPrice := int64(0)
	for _, item := range bootstrap.Items {
		if !textSliceContains(item.Modes, defaultMode) {
			return fmt.Errorf("database item %q modes must include %q", item.ID, defaultMode)
		}
		/*
		 * Price 是这张卡真正参与游戏金额计算的数值。无论它是消费卡还是 income 返钱卡，
		 * 都必须是一个正数：消费卡用它扣余额，income 卡用它加余额。0 元卡看起来不会
		 * 立刻崩溃，但前端仍会把它当成一张可选择的卡，它会占用 9 张货架的位置和一次
		 * 结算节奏，却不会推动余额变化，最后让“7 分钟左右清空余额”的抽卡算法失真。
		 * 因此这里不只检查整体内容包里有没有正价卡，而是要求每一张进入前端的卡都有
		 * 明确金额。
		 */
		if item.Price <= 0 {
			return fmt.Errorf("database item %q price must be positive", item.ID)
		}
		/*
		 * weight 和 minBalance 会直接进入前端抽牌算法。weight 是这张商品在候选池里的
		 * 基础权重，数值越大越容易出现在 9 张货架里；minBalance 是余额门槛，表示玩家
		 * 至少还剩多少钱时才允许这张卡进入候选。前端会继续叠加场景、价位和节奏权重，
		 * 但它不会把一个错误的负门槛或非正权重重新解释成合理配置。这里先在 Go 内容包
		 * 边界拒绝坏数据，避免数据库旧表约束缺失时，浏览器拿到会推偏金额算法的商品卡。
		 */
		if item.Weight <= 0 {
			return fmt.Errorf("database item %q weight must be positive", item.ID)
		}
		if item.MinBalance < 0 {
			return fmt.Errorf("database item %q minBalance must not be negative", item.ID)
		}
		/*
		 * MaxBuy 是后端给前端的一条次数上限约定。它为空时表示这个商品没有次数限制；
		 * 它有值时必须是正数，前端才会把它当成“本局最多可购买几次”。如果数据库误写成
		 * 0 或负数，前端为了避免崩溃会把这个值当成没有限制，这会让原本一次性的高额卡
		 * 悄悄变成可重复购买。这里在内容包发出前拦住坏值，让数据库、Go API 和浏览器
		 * 对同一个字段保持相同语义。
		 */
		if item.MaxBuy != nil && *item.MaxBuy <= 0 {
			return fmt.Errorf("database item %q maxBuy must be positive when present", item.ID)
		}

		/*
		 * 商品非空还不等于游戏可玩。前端的目标是把初始余额花光，所以至少要有一张在
		 * 开局余额范围内能直接支付的消费卡；否则画面可以发出 9 张卡，但玩家第一手就
		 * 可能全是买不起的内容。income 卡则承接退款、赔付、中奖这条反向压力线，它们
		 * 会把钱加回来，让“尽快清空”不是单纯线性扣款。如果这些卡被误删，核心玩法会
		 * 悄悄缩水，因此也在内容包边界提前拦住。
		 */
		if item.Category != "" {
			seenItemCategories[item.Category] = struct{}{}
		}
		if item.SceneID != nil && *item.SceneID != "" {
			if _, exists := sceneIDs[*item.SceneID]; !exists {
				return fmt.Errorf("database item %q references missing scene %q", item.ID, *item.SceneID)
			}
			seenItemSceneLinks[*item.SceneID] = struct{}{}
		}
		if item.Tier != "" {
			seenItemTiers[item.Tier] = struct{}{}
			seenItemTierCounts[item.Tier] += 1
		}
		for _, tag := range item.Tags {
			availableEventTags[tag] = struct{}{}
		}
		if item.Tier == "income" {
			if item.Price > 0 {
				hasIncomeItem = true
			}
			continue
		}
		if item.Price > 0 && (lowestSpendItemPrice == 0 || item.Price < lowestSpendItemPrice) {
			lowestSpendItemPrice = item.Price
		}
		if item.Price <= 50 {
			changeSpendItemCount += 1
		}
		if item.Price > highestSpendItemPrice {
			highestSpendItemPrice = item.Price
		}
		if item.Price > 0 && item.MinBalance <= bootstrap.Config.InitialBalance && item.Price <= bootstrap.Config.InitialBalance {
			hasPayableSpendItem = true
		}
	}
	if !hasPayableSpendItem {
		return fmt.Errorf("database content must include at least one spend item payable at the initial balance")
	}
	if !hasIncomeItem {
		return fmt.Errorf("database content must include at least one income item")
	}
	if len(seenItemCategories) < minimumBootstrapItemCategories {
		return fmt.Errorf("database content item categories = %d, want at least %d", len(seenItemCategories), minimumBootstrapItemCategories)
	}
	if len(seenItemSceneLinks) < minimumBootstrapItemScenes {
		return fmt.Errorf("database content item scene links = %d, want at least %d", len(seenItemSceneLinks), minimumBootstrapItemScenes)
	}

	/*
	 * “覆盖各个价位”不能只靠人工看 seed.sql。前端的找零阶段、大额消费概率、倍率限制和
	 * 追赶卡算法都依赖 tier 与价格层存在：coin/small/daily 负责低余额和普通货架，
	 * premium/large/heavy/shock 负责中高压消费，income 负责返钱反向压力。这里不只检查
	 * “有没有某一层”，还要求每层至少有几张卡，因为只有一张 small 或 shock 也会让抽卡
	 * 权重变成摆设，前端看起来有档位，实际发牌时却几乎抽不到。
	 *
	 * 这里的最低价和最高价必须只看消费卡，不能把 income 卡混进去。income 卡代表退款、
	 * 补贴或赔付，玩家拿到它时余额会上升，并不能帮玩家在低余额阶段继续清空余额。如果
	 * 只剩 50 元返钱卡而没有 50 元消费卡，前端仍会进入找零阶段，但玩家看到的是返钱而
	 * 不是可支付的小额账单；同理，30 万以上的高价收入也不能证明冲击消费层仍然存在。
	 */
	requiredItemTiers := []string{"coin", "small", "daily", "premium", "large", "heavy", "shock", "income"}
	for _, tier := range requiredItemTiers {
		if _, exists := seenItemTiers[tier]; !exists {
			return fmt.Errorf("database content must include item tier %q", tier)
		}
		if seenItemTierCounts[tier] < minimumBootstrapTierItems {
			return fmt.Errorf("database content item tier %q count = %d, want at least %d", tier, seenItemTierCounts[tier], minimumBootstrapTierItems)
		}
	}
	if lowestSpendItemPrice > 1 {
		return fmt.Errorf("database content lowest spend item price = %d, want a 1-yuan spend card for the change stage", lowestSpendItemPrice)
	}
	if changeSpendItemCount < minimumChangeSpendItems {
		return fmt.Errorf("database content change spend item count = %d, want at least %d cards at or below 50", changeSpendItemCount, minimumChangeSpendItems)
	}
	if highestSpendItemPrice < 300_000 {
		return fmt.Errorf("database content highest spend item price = %d, want shock-level spend cards at or above 300000", highestSpendItemPrice)
	}

	hasSpendEvent := false
	hasIncomeEvent := false
	maximumChaosEventDelta := bootstrap.Config.InitialBalance / maximumChaosEventDeltaDivisor
	if maximumChaosEventDelta < 1 {
		maximumChaosEventDelta = 1
	}
	for _, event := range bootstrap.Events {
		if !textSliceContains(event.Modes, defaultMode) {
			return fmt.Errorf("database event %q modes must include %q", event.ID, defaultMode)
		}

		/*
		 * event.delta 是随机事件对余额的直接影响：负数表示额外扣钱，例如罚单、损坏费、
		 * 加急费；正数表示返钱或赔付，例如押金退回、保险理赔、平台补贴。前端在购买后
		 * 会根据这个数字更新余额和战报。如果数据库只剩无金额提示事件，玩家会看到流水，
		 * 但“意外成本”和“返钱阻碍清空”两条核心压力都会消失；如果只剩单方向事件，
		 * 长局节奏也会偏向单调扣款或单调返钱。所以这里要求两个方向至少各有一条。
		 */
		if event.Probability <= 0 || event.Probability > maximumChaosEventProbability {
			return fmt.Errorf("database event %q probability %.4f outside allowed range", event.ID, event.Probability)
		}
		if event.CooldownSec < 0 {
			return fmt.Errorf("database event %q has negative cooldown", event.ID)
		}
		if len(event.Tags) == 0 {
			return fmt.Errorf("database event %q must include matching tags", event.ID)
		}
		hasMatchingEventTag := false
		for _, tag := range event.Tags {
			if _, exists := availableEventTags[tag]; exists {
				hasMatchingEventTag = true
				break
			}
		}
		if !hasMatchingEventTag {
			return fmt.Errorf("database event %q has no tag matching items or scenes", event.ID)
		}

		if event.Delta == nil {
			continue
		}
		if *event.Delta < -maximumChaosEventDelta || *event.Delta > maximumChaosEventDelta {
			return fmt.Errorf("database event %q delta = %d outside allowed range", event.ID, *event.Delta)
		}
		switch {
		case *event.Delta < 0:
			hasSpendEvent = true
		case *event.Delta > 0:
			hasIncomeEvent = true
		}
	}
	if !hasSpendEvent {
		return fmt.Errorf("database content must include at least one spend event")
	}
	if !hasIncomeEvent {
		return fmt.Errorf("database content must include at least one income event")
	}

	hasRefreshStatus := false
	hasHighPriceStatus := false
	hasEventStatus := false
	for _, effect := range bootstrap.Statuses {
		/*
		 * 状态效果不是单纯显示一行“心情变化”。前端会把 itemRefreshMultiplier 用在货架
		 * 刷新速度上，把 highPriceMultiplier 用在大额卡权重上，把 eventMultiplier 用在
		 * 普通混沌事件概率上。倍率必须落在一个现实范围内，否则会出现货架刷新过快、
		 * 大额卡权重爆炸或随机事件几乎必出的情况；同时内容包至少要覆盖这三类变化，
		 * 才符合开发计划里“状态影响商品概率和事件概率”的机制。
		 */
		if effect.ItemRefreshMultiplier <= 0 || effect.HighPriceMultiplier <= 0 || effect.EventMultiplier <= 0 {
			return fmt.Errorf("database status %q has non-positive multiplier", effect.ID)
		}
		if effect.DurationSec < minimumStatusDurationSec || effect.DurationSec > maximumStatusDurationSec {
			return fmt.Errorf("database status %q durationSec = %d outside allowed range", effect.ID, effect.DurationSec)
		}
		if effect.ItemRefreshMultiplier < 0.5 || effect.ItemRefreshMultiplier > 1.8 {
			return fmt.Errorf("database status %q itemRefreshMultiplier %.2f outside allowed range", effect.ID, effect.ItemRefreshMultiplier)
		}
		if effect.HighPriceMultiplier < 0.5 || effect.HighPriceMultiplier > 2 {
			return fmt.Errorf("database status %q highPriceMultiplier %.2f outside allowed range", effect.ID, effect.HighPriceMultiplier)
		}
		if effect.EventMultiplier < 0.5 || effect.EventMultiplier > 1.8 {
			return fmt.Errorf("database status %q eventMultiplier %.2f outside allowed range", effect.ID, effect.EventMultiplier)
		}
		if len(effect.Tags) == 0 {
			return fmt.Errorf("database status %q must include matching tags", effect.ID)
		}
		if effect.ItemRefreshMultiplier != 1 {
			hasRefreshStatus = true
		}
		if effect.HighPriceMultiplier != 1 {
			hasHighPriceStatus = true
		}
		if effect.EventMultiplier != 1 {
			hasEventStatus = true
		}
	}
	if !hasRefreshStatus {
		return fmt.Errorf("database content must include at least one status that changes hand refresh speed")
	}
	if !hasHighPriceStatus {
		return fmt.Errorf("database content must include at least one status that changes high-price weighting")
	}
	if !hasEventStatus {
		return fmt.Errorf("database content must include at least one status that changes event probability")
	}

	availableTerminalTags := make(map[string]struct{})
	for _, item := range bootstrap.Items {
		for _, tag := range item.Tags {
			availableTerminalTags[tag] = struct{}{}
		}
	}
	maxSceneRiskLevel := int64(0)
	for _, scene := range bootstrap.Scenes {
		if scene.RiskLevel > maxSceneRiskLevel {
			maxSceneRiskLevel = scene.RiskLevel
		}
		for _, tag := range scene.ItemTags {
			availableTerminalTags[tag] = struct{}{}
		}
		for _, tag := range scene.EventTags {
			availableTerminalTags[tag] = struct{}{}
		}
	}

	hasTriggerableEnding := false
	hasTriggerableZeroEnding := false
	for _, ending := range bootstrap.Endings {
		if strings.TrimSpace(ending.ID) == "" || strings.TrimSpace(ending.Title) == "" || strings.TrimSpace(ending.Description) == "" {
			return fmt.Errorf("database terminal event has empty identity")
		}
		if !textSliceContains(ending.Modes, defaultMode) {
			return fmt.Errorf("database terminal event %q modes must include %q", ending.ID, defaultMode)
		}
		if ending.Probability <= 0 || ending.Probability > maxTerminalEventProbability {
			return fmt.Errorf("database terminal event %q probability %.4f outside allowed range", ending.ID, ending.Probability)
		}
		if ending.MinElapsedMs < 0 || ending.MinElapsedMs >= bootstrap.Config.RoundLimitMs {
			return fmt.Errorf("database terminal event %q minElapsedMs = %d outside playable round", ending.ID, ending.MinElapsedMs)
		}
		if ending.MaxBalance != nil && *ending.MaxBalance <= 0 {
			return fmt.Errorf("database terminal event %q maxBalance must be positive when present", ending.ID)
		}
		if ending.MinRiskLevel < 1 || ending.MinRiskLevel > 5 {
			return fmt.Errorf("database terminal event %q minRiskLevel = %d outside allowed range", ending.ID, ending.MinRiskLevel)
		}
		if ending.BalanceEffect != "none" && ending.BalanceEffect != "zero" {
			return fmt.Errorf("database terminal event %q has unsupported balanceEffect %q", ending.ID, ending.BalanceEffect)
		}

		/*
		 * 终局事件不是普通文案。前端只有在购买消费卡之后，才会检查终局事件的时间门槛、
		 * 余额上限、场景风险和标签匹配；通过后才会提前停表并生成特殊战报。如果数据库里的
		 * 终局全都超过硬结算时间、要求不存在的标签，或者风险等级高过所有场景，API 虽然能
		 * 返回 endings，但一整局永远触发不了。这里用和前端同方向的基础条件做服务端兜底。
		 */
		if !terminalEventCanEverTrigger(ending, bootstrap.Config.RoundLimitMs, maxSceneRiskLevel, availableTerminalTags) {
			continue
		}
		hasTriggerableEnding = true
		if ending.BalanceEffect == "zero" {
			hasTriggerableZeroEnding = true
		}
	}
	if !hasTriggerableEnding {
		return fmt.Errorf("database content must include at least one triggerable terminal event")
	}
	if !hasTriggerableZeroEnding {
		return fmt.Errorf("database content must include at least one triggerable zero-balance terminal event")
	}

	allowedAudioMoods := map[string]struct{}{"menu": {}, "rush": {}, "danger": {}, "settlement": {}}
	allowedAudioLicenses := map[string]struct{}{"CC0": {}, "MIT": {}, "custom": {}}
	for _, track := range bootstrap.AudioTracks {
		/*
		 * AudioTrack 会原样发给浏览器端的 AudioDirector。src 现在可以为空，表示继续使用前端
		 * 内置合成音乐；等后续拿到真实授权音乐时，src 才会指向音频文件。license 不能随便
		 * 写字符串，因为前端类型只接受 CC0、MIT 和 custom，这三种分别表示公共领域/宽松开源/
		 * 项目自有或另行授权。这里先把身份、情绪和授权字段收紧，避免数据库返回一条前端类型
		 * 之外的音轨，后续做音乐来源展示或素材审计时也不会出现无意义 license。
		 */
		if strings.TrimSpace(track.ID) == "" || strings.TrimSpace(track.Title) == "" {
			return fmt.Errorf("database audio track has empty identity")
		}
		if _, exists := allowedAudioMoods[track.Mood]; !exists {
			return fmt.Errorf("database audio track %q has unsupported mood %q", track.ID, track.Mood)
		}
		if _, exists := allowedAudioLicenses[track.License]; !exists {
			return fmt.Errorf("database audio track %q has unsupported license %q", track.ID, track.License)
		}
	}

	return nil
}

func terminalEventCanEverTrigger(ending terminalEvent, roundLimitMs int64, maxSceneRiskLevel int64, availableTags map[string]struct{}) bool {
	if ending.Probability <= 0 || ending.MinElapsedMs >= roundLimitMs || ending.MinRiskLevel > maxSceneRiskLevel {
		return false
	}
	if ending.MaxBalance != nil && *ending.MaxBalance <= 0 {
		return false
	}

	hasMeaningfulTag := false
	for _, tag := range ending.Tags {
		if tag == "ending" {
			continue
		}
		hasMeaningfulTag = true
		if _, exists := availableTags[tag]; exists {
			return true
		}
	}

	return !hasMeaningfulTag
}

func (state *appState) loadItems(ctx context.Context) ([]item, error) {
	rows, err := state.db.QueryContext(ctx, `
		SELECT id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance,
		       modes::text, tags::text, flavor
		FROM content_items
		WHERE active = true AND modes ? $1
		ORDER BY sort_order, id
	`, defaultMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]item, 0, 128)
	for rows.Next() {
		var next item
		var sceneID sql.NullString
		var maxBuy sql.NullInt64
		var modesJSON string
		var tagsJSON string

		if err := rows.Scan(
			&next.ID,
			&next.Name,
			&next.Category,
			&sceneID,
			&next.Price,
			&next.Tier,
			&maxBuy,
			&next.Batchable,
			&next.Weight,
			&next.MinBalance,
			&modesJSON,
			&tagsJSON,
			&next.Flavor,
		); err != nil {
			return nil, err
		}

		if sceneID.Valid {
			next.SceneID = ptrString(sceneID.String)
		}
		if maxBuy.Valid {
			next.MaxBuy = ptrInt64(maxBuy.Int64)
		}
		next.Modes = decodeTextArrayJSON(modesJSON)
		next.Tags = decodeTextArrayJSON(tagsJSON)
		items = append(items, next)
	}

	return items, rows.Err()
}

func (state *appState) loadScenes(ctx context.Context) ([]scene, error) {
	rows, err := state.db.QueryContext(ctx, `
		SELECT id, name, entry_cost, duration_sec, min_balance, rarity, risk_level,
		       item_tags::text, event_tags::text, modes::text
		FROM content_scenes
		WHERE active = true AND modes ? $1
		ORDER BY sort_order, id
	`, defaultMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scenes := make([]scene, 0, 32)
	for rows.Next() {
		var next scene
		var itemTagsJSON string
		var eventTagsJSON string
		var modesJSON string

		if err := rows.Scan(
			&next.ID,
			&next.Name,
			&next.EntryCost,
			&next.DurationSec,
			&next.MinBalance,
			&next.Rarity,
			&next.RiskLevel,
			&itemTagsJSON,
			&eventTagsJSON,
			&modesJSON,
		); err != nil {
			return nil, err
		}

		next.ItemTags = decodeTextArrayJSON(itemTagsJSON)
		next.EventTags = decodeTextArrayJSON(eventTagsJSON)
		next.Modes = decodeTextArrayJSON(modesJSON)
		scenes = append(scenes, next)
	}

	return scenes, rows.Err()
}

func (state *appState) loadEvents(ctx context.Context) ([]gameEvent, error) {
	rows, err := state.db.QueryContext(ctx, `
		SELECT id, title, description, delta, probability, cooldown_sec, tags::text, modes::text, settlement_tag
		FROM content_events
		WHERE active = true AND modes ? $1
		ORDER BY sort_order, id
	`, defaultMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]gameEvent, 0, 64)
	for rows.Next() {
		var next gameEvent
		var delta sql.NullInt64
		var tagsJSON string
		var modesJSON string

		if err := rows.Scan(
			&next.ID,
			&next.Title,
			&next.Description,
			&delta,
			&next.Probability,
			&next.CooldownSec,
			&tagsJSON,
			&modesJSON,
			&next.SettlementTag,
		); err != nil {
			return nil, err
		}

		if delta.Valid {
			next.Delta = ptrDelta(delta.Int64)
		}
		next.Tags = decodeTextArrayJSON(tagsJSON)
		next.Modes = decodeTextArrayJSON(modesJSON)
		events = append(events, next)
	}

	return events, rows.Err()
}

func (state *appState) loadEndings(ctx context.Context) ([]terminalEvent, error) {
	rows, err := state.db.QueryContext(ctx, `
		SELECT id, title, description, probability, min_elapsed_ms, max_balance, min_risk_level,
		       balance_effect, tags::text, modes::text, settlement_tag
		FROM content_endings
		WHERE active = true AND modes ? $1
		ORDER BY sort_order, id
	`, defaultMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	endings := make([]terminalEvent, 0, 8)
	for rows.Next() {
		var next terminalEvent
		var maxBalance sql.NullInt64
		var tagsJSON string
		var modesJSON string

		if err := rows.Scan(
			&next.ID,
			&next.Title,
			&next.Description,
			&next.Probability,
			&next.MinElapsedMs,
			&maxBalance,
			&next.MinRiskLevel,
			&next.BalanceEffect,
			&tagsJSON,
			&modesJSON,
			&next.SettlementTag,
		); err != nil {
			return nil, err
		}

		if maxBalance.Valid {
			next.MaxBalance = ptrInt64(maxBalance.Int64)
		}
		next.Tags = decodeTextArrayJSON(tagsJSON)
		next.Modes = decodeTextArrayJSON(modesJSON)
		endings = append(endings, next)
	}

	return endings, rows.Err()
}

func (state *appState) loadStatuses(ctx context.Context) ([]statusEffect, error) {
	rows, err := state.db.QueryContext(ctx, `
		SELECT id, name, duration_sec, item_refresh_multiplier, high_price_multiplier, event_multiplier, tags::text, description
		FROM content_statuses
		WHERE active = true
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make([]statusEffect, 0, 16)
	for rows.Next() {
		var next statusEffect
		var tagsJSON string
		if err := rows.Scan(
			&next.ID,
			&next.Name,
			&next.DurationSec,
			&next.ItemRefreshMultiplier,
			&next.HighPriceMultiplier,
			&next.EventMultiplier,
			&tagsJSON,
			&next.Description,
		); err != nil {
			return nil, err
		}
		next.Tags = decodeTextArrayJSON(tagsJSON)
		statuses = append(statuses, next)
	}

	return statuses, rows.Err()
}

func (state *appState) loadAudioTracks(ctx context.Context) ([]audioTrack, error) {
	rows, err := state.db.QueryContext(ctx, `
		SELECT id, title, mood, src, license, source_url
		FROM audio_tracks
		WHERE active = true
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]audioTrack, 0, 8)
	for rows.Next() {
		var next audioTrack
		if err := rows.Scan(&next.ID, &next.Title, &next.Mood, &next.Src, &next.License, &next.SourceURL); err != nil {
			return nil, err
		}
		tracks = append(tracks, next)
	}

	return tracks, rows.Err()
}

func decodeTextArrayJSON(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return []string{}
	}
	return values
}

func textSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func (state *appState) reserveUsernameInDatabase(ctx context.Context, username string, providedToken string) (bool, string, error) {
	tx, err := state.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM usernames AS user_record
		WHERE user_record.username = $1
		  AND (user_record.reserved_until IS NULL OR user_record.reserved_until < now())
		  AND NOT EXISTS (
		    SELECT 1
		    FROM runs
		    WHERE runs.username = user_record.username
		  )
	`, username); err != nil {
		return false, "", err
	}

	token, err := newReservationToken()
	if err != nil {
		return false, "", err
	}
	reservedUntil := time.Now().Add(usernameReservationTTL)

	var reservationToken string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO usernames (username, reservation_token, reserved_until)
		VALUES ($1, $2, $3)
		ON CONFLICT (username) DO NOTHING
		RETURNING reservation_token
	`, username, token, reservedUntil).Scan(&reservationToken)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return false, "", err
		}
		return true, reservationToken, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, "", err
	}

	var existingToken string
	var hasSubmittedRun bool
	err = tx.QueryRowContext(ctx, `
		SELECT usernames.reservation_token,
		       EXISTS (
		         SELECT 1
		         FROM runs
		         WHERE runs.username = usernames.username
		       ) AS has_submitted_run
		FROM usernames
		WHERE usernames.username = $1
		FOR UPDATE
	`, username).Scan(&existingToken, &hasSubmittedRun)
	if err != nil {
		return false, "", err
	}
	if hasSubmittedRun {
		if err := tx.Commit(); err != nil {
			return false, "", err
		}
		return false, "", nil
	}
	if providedToken == "" || existingToken == "" || providedToken != existingToken {
		if err := tx.Commit(); err != nil {
			return false, "", err
		}
		return false, "", nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE usernames
		SET reserved_until = $2
		WHERE username = $1
	`, username, reservedUntil); err != nil {
		return false, "", err
	}
	if err := tx.Commit(); err != nil {
		return false, "", err
	}

	return true, existingToken, nil
}

func (state *appState) submitRunToDatabase(ctx context.Context, run runSubmission) (leaderboardEntry, error) {
	if err := validateRun(run); err != nil {
		return leaderboardEntry{}, err
	}

	tx, err := state.db.BeginTx(ctx, nil)
	if err != nil {
		return leaderboardEntry{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM usernames AS user_record
		WHERE user_record.username = $1
		  AND user_record.reservation_token <> ''
		  AND (user_record.reserved_until IS NULL OR user_record.reserved_until < now())
		  AND NOT EXISTS (
		    SELECT 1
		    FROM runs
		    WHERE runs.username = user_record.username
		  )
	`, run.Username); err != nil {
		return leaderboardEntry{}, err
	}

	// 成绩提交可能发生在用户名刚占用之后，也可能因为网络重试直接打到提交接口。
	// 这里先确保 usernames 有这条记录，再插入 runs。runs.username 有唯一约束，
	// 因此同一个用户名不会产生多条排行榜成绩；并发请求也交给数据库唯一索引处理，
	// 不需要 Go 里再套一把容易误用的全局锁。
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usernames (username)
		VALUES ($1)
		ON CONFLICT (username) DO NOTHING
	`, run.Username); err != nil {
		return leaderboardEntry{}, err
	}
	if err := state.ensureRunMatchesReservation(ctx, tx, run); err != nil {
		return leaderboardEntry{}, err
	}

	storedContentVersion := normalizeContentVersion(run.ContentVersion)
	rankingContentVersion := optionalContentVersion(run.ContentVersion)
	var runID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO runs (
			username, duration_ms, max_single_spend, final_balance, total_spent, total_income, ended_by, chaos_seed,
			content_version, ending_id, ending_title, ending_detail
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''))
		ON CONFLICT (username) DO NOTHING
		RETURNING id
	`, run.Username, run.DurationMs, run.MaxSingleSpend, run.FinalBalance, run.TotalSpent, run.TotalIncome, run.EndedBy, run.ChaosSeed, storedContentVersion, run.EndingID, run.EndingTitle, run.EndingDetail).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return leaderboardEntry{}, errDuplicateRun
	}
	if err != nil {
		return leaderboardEntry{}, err
	}

	// 排行榜名次必须和成绩写入使用同一个事务边界。否则会出现一种尴尬状态：
	// INSERT 已经提交成功，但提交后的第二次查询因为网络抖动或 context 超时失败，
	// 前端收到 500，以为成绩没有写入；用户再次提交时又会被唯一索引判定为重复成绩。
	// 在事务里先查出当前名次，只有这一步也成功才提交，可以避免“数据库已经保存，
	// 但 API 响应失败”的不一致窗口。
	entry, err := leaderboardEntryForUsername(ctx, tx, run.Username, rankingContentVersion)
	if err != nil {
		return leaderboardEntry{}, err
	}

	if err := tx.Commit(); err != nil {
		return leaderboardEntry{}, err
	}

	return entry, nil
}

func (state *appState) ensureRunMatchesReservation(ctx context.Context, tx *sql.Tx, run runSubmission) error {
	var reservationToken string
	var reservedUntil sql.NullTime
	var hasSubmittedRun bool
	err := tx.QueryRowContext(ctx, `
		SELECT usernames.reservation_token,
		       usernames.reserved_until,
		       EXISTS (
		         SELECT 1
		         FROM runs
		         WHERE runs.username = usernames.username
		       ) AS has_submitted_run
		FROM usernames
		WHERE usernames.username = $1
		FOR UPDATE
	`, run.Username).Scan(&reservationToken, &reservedUntil, &hasSubmittedRun)
	if err != nil {
		return err
	}
	if hasSubmittedRun {
		return errDuplicateRun
	}
	if reservationToken == "" {
		return nil
	}
	if reservedUntil.Valid && time.Now().Before(reservedUntil.Time) && run.ReservationToken != reservationToken {
		return errUsernameReserved
	}

	return nil
}

func (state *appState) leaderboardEntryForUsername(ctx context.Context, username string) (leaderboardEntry, error) {
	return leaderboardEntryForUsername(ctx, state.db, username, "")
}

type leaderboardEntryQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func leaderboardEntryForUsername(ctx context.Context, querier leaderboardEntryQuerier, username string, contentVersion string) (leaderboardEntry, error) {
	var entry leaderboardEntry
	err := querier.QueryRowContext(ctx, `
		WITH ranked AS (
			SELECT
				row_number() OVER (
					ORDER BY (final_balance = 0) DESC, duration_ms ASC, max_single_spend DESC, created_at ASC, id ASC
				) AS rank,
				username,
				duration_ms,
				max_single_spend
			FROM runs
			WHERE ($1 = '' OR content_version = $1)
		)
		SELECT rank, username, duration_ms, max_single_spend
		FROM ranked
		WHERE username = $2
	`, contentVersion, username).Scan(&entry.Rank, &entry.Username, &entry.DurationMs, &entry.MaxSingleSpend)
	return entry, err
}

func (state *appState) leaderboardFromDatabase(ctx context.Context, limit int, contentVersion string) ([]leaderboardEntry, error) {
	rows, err := state.db.QueryContext(ctx, `
		WITH ranked AS (
			SELECT
				row_number() OVER (
					ORDER BY (final_balance = 0) DESC, duration_ms ASC, max_single_spend DESC, created_at ASC, id ASC
				) AS rank,
				username,
				duration_ms,
				max_single_spend
			FROM runs
			WHERE ($1 = '' OR content_version = $1)
		)
		SELECT rank, username, duration_ms, max_single_spend
		FROM ranked
		ORDER BY rank
		LIMIT $2
	`, contentVersion, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]leaderboardEntry, 0, limit)
	for rows.Next() {
		var entry leaderboardEntry
		if err := rows.Scan(&entry.Rank, &entry.Username, &entry.DurationMs, &entry.MaxSingleSpend); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
