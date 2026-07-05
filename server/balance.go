package main

import (
	"fmt"
	"sort"
)

const (
	defaultInitialBalance     int64 = 2_500_000
	defaultRoundLimitMs       int64 = 660_000
	targetStageCount          int64 = 12
	targetClearMs             int64 = 420_000
	minHandRefreshMs          int64 = 6_500
	maxHandRefreshMs          int64 = 9_000
	defaultInterestIntervalMs int64 = 10_000
	defaultInterestDelayMs    int64 = 60_000
	defaultSelectionSettleMs  int64 = 1_700
	defaultClearCartDelayMs   int64 = 7_000
	visibleCardCount          int64 = 9
)

// deriveBalanceTuning 是当前后端的第一层“金额和平衡算法”。这里没有引入复杂框架，
// 只是把前端原本散落的秒数、概率和倍率限制收回到后端内容包里。这样前端启动一局时，
// 拿到的不只是商品列表，还能拿到“这一局应该按什么节奏刷新、什么倍率能出现、技能要
// 延迟多久、事件概率如何随场景风险变化”的规则。
//
// 这里的算法故意保持透明：主节奏目标是约 7 分钟花完，被拆成 12 个阶段，所以一个阶段
// 大约 35 秒。roundLimitMs 是硬结束时间，当前为 11 分钟，用来承接“少花钱多挣钱”的
// 隐藏路线；它不是抽卡和技能冷却的主要节奏来源。
// VISA 冷却和购物车冷却都绑定阶段长度，避免用户一直点技能几秒结束；VISA 延迟取阶段
// 的三分之二，购物车延迟现在固定为 7 秒，形成两个不同的等待压力。直接点选会有 1.7 秒
// 收银结算锁，这不是为了惩罚玩家，而是为了让“读卡并选择更合适消费”比“盲点同一位置”
// 更有价值。倍率规则则按初始余额推导金额上限：倍率越高，允许使用的单价和总价越低，
// 避免 x10 或 x20 直接把游戏变成一次按钮清空。
func deriveBalanceTuning(initialBalance int64, _ int64) balanceTuning {
	stageCount := targetStageCount
	stageDurationMs := targetClearMs / stageCount
	handRefreshMs := clampInt64(stageDurationMs/6, minHandRefreshMs, maxHandRefreshMs)
	highPriceThreshold := initialBalance / 160

	return balanceTuning{
		StageCount:           stageCount,
		StageDurationMs:      stageDurationMs,
		TargetClearMs:        targetClearMs,
		HandRefreshMs:        handRefreshMs,
		SelectionSettleMs:    defaultSelectionSettleMs,
		InterestStartDelayMs: defaultInterestDelayMs,
		InterestIntervalMs:   defaultInterestIntervalMs,
		InterestRate:         0.03,
		InterestBands: []interestBand{
			{MinBalance: initialBalance * 4 / 5, Rate: 0.012},
			{MinBalance: initialBalance / 2, Rate: 0.018},
			{MinBalance: initialBalance / 5, Rate: 0.028},
			{MinBalance: initialBalance / 20, Rate: 0.042},
			{MinBalance: 0, Rate: 0.065},
		},
		VisaDelayMs:              stageDurationMs * 2 / 3,
		VisaCooldownMs:           stageDurationMs,
		ClearCartDelayMs:         defaultClearCartDelayMs,
		ClearCartCooldownMs:      stageDurationMs,
		ClearCartPickCount:       3,
		NormalHighCardHandChance: 0.03,
		SpecialHighCardCount:     2,
		HighPriceThreshold:       highPriceThreshold,
		EventBaseChance:          0.18,
		EventRiskBonus:           0.025,
		EventMatchBonus:          0.08,
		MultiplierRules: []multiplierRule{
			{
				ID:            "x1",
				Label:         "x1",
				Multiplier:    1,
				MinBalance:    0,
				MaxUnitPrice:  initialBalance,
				MaxTotalPrice: initialBalance,
				Weight:        18,
			},
			{
				ID:            "x3",
				Label:         "x3",
				Multiplier:    3,
				MinBalance:    initialBalance / 20,
				MaxUnitPrice:  initialBalance / 1_000,
				MaxTotalPrice: initialBalance / 180,
				Weight:        5,
			},
			{
				ID:            "x5",
				Label:         "x5",
				Multiplier:    5,
				MinBalance:    initialBalance / 5,
				MaxUnitPrice:  initialBalance / 2_200,
				MaxTotalPrice: initialBalance / 220,
				Weight:        2,
			},
			{
				ID:            "x10",
				Label:         "x10",
				Multiplier:    10,
				MinBalance:    initialBalance / 2,
				MaxUnitPrice:  initialBalance / 8_000,
				MaxTotalPrice: initialBalance / 600,
				Weight:        0.75,
			},
			{
				ID:            "x20",
				Label:         "x20",
				Multiplier:    20,
				MinBalance:    initialBalance * 4 / 5,
				MaxUnitPrice:  initialBalance / 20_000,
				MaxTotalPrice: initialBalance / 900,
				Weight:        0.25,
			},
		},
	}
}

func clampInt64(value int64, minValue int64, maxValue int64) int64 {
	if value < minValue {
		return minValue
	}

	if value > maxValue {
		return maxValue
	}

	return value
}

func validateBalanceTuning(config gameConfig) error {
	tuning := config.BalanceTuning
	if config.InitialBalance <= 0 || config.RoundLimitMs <= 0 {
		return fmt.Errorf("game config must include positive initial balance and round limit")
	}
	if config.DefaultMode != defaultMode {
		return fmt.Errorf("game config default mode = %q, want %q", config.DefaultMode, defaultMode)
	}

	/*
	 * balanceTuning 是前端真正执行时间和金额算法的规则包。这里校验的不是“数字看起来
	 * 不为空”，而是这些数字能不能组成一局可玩的游戏：12 阶段要覆盖 7 分钟主线目标，
	 * 货架刷新必须比收银锁更长，第一笔利息要在硬结算之前发生，技能延迟和冷却不能为 0，
	 * 购物车选择数量不能超过一手 9 张卡。这样后端调参出错时，API 会直接暴露内容包错误，
	 * 而不是让浏览器拿着一组会破坏玩法的数字开局。
	 */
	if tuning.StageCount <= 0 || tuning.StageDurationMs <= 0 || tuning.TargetClearMs <= 0 {
		return fmt.Errorf("balance tuning stage timing must be positive")
	}
	if tuning.StageDurationMs*tuning.StageCount < tuning.TargetClearMs {
		return fmt.Errorf("balance tuning stages must cover target clear time")
	}
	if tuning.TargetClearMs >= config.RoundLimitMs {
		return fmt.Errorf("balance tuning target clear time must be before hard round limit")
	}
	if tuning.HandRefreshMs <= 0 || tuning.SelectionSettleMs <= 0 || tuning.HandRefreshMs <= tuning.SelectionSettleMs {
		return fmt.Errorf("balance tuning hand refresh must be longer than checkout settle lock")
	}
	if tuning.InterestStartDelayMs < 0 || tuning.InterestIntervalMs <= 0 || tuning.InterestRate <= 0 || tuning.InterestRate >= 1 {
		return fmt.Errorf("balance tuning interest settings are invalid")
	}
	if tuning.InterestStartDelayMs+tuning.InterestIntervalMs >= config.RoundLimitMs {
		return fmt.Errorf("balance tuning first interest payment must happen before hard round limit")
	}
	if len(tuning.InterestBands) == 0 {
		return fmt.Errorf("balance tuning must include interest bands")
	}
	hasZeroInterestBand := false
	for _, band := range tuning.InterestBands {
		if band.MinBalance < 0 || band.Rate <= 0 || band.Rate >= 1 {
			return fmt.Errorf("balance tuning interest band is invalid")
		}
		if band.MinBalance == 0 {
			hasZeroInterestBand = true
		}
	}
	if !hasZeroInterestBand {
		return fmt.Errorf("balance tuning must include a zero-balance interest band")
	}
	sortedInterestBands := append([]interestBand(nil), tuning.InterestBands...)
	sort.Slice(sortedInterestBands, func(firstIndex int, secondIndex int) bool {
		return sortedInterestBands[firstIndex].MinBalance > sortedInterestBands[secondIndex].MinBalance
	})
	for index := 1; index < len(sortedInterestBands); index += 1 {
		previousBand := sortedInterestBands[index-1]
		currentBand := sortedInterestBands[index]
		if currentBand.MinBalance == previousBand.MinBalance {
			continue
		}

		/*
		 * 利息是反向压力：玩家余额越少，越应该被更高百分比的利息拖住，而不是余额越少
		 * 越轻松。前端会直接按这些档位计算下一笔入账，如果后端内容包把 0 元档写成更低
		 * 利率，浏览器不会知道这是调参错误，只会照着执行。这里按余额门槛从高到低检查，
		 * 保证低余额档位的利率不会低于高余额档位。
		 */
		if currentBand.Rate < previousBand.Rate {
			return fmt.Errorf("balance tuning lower-balance interest rate must not be below higher-balance rate")
		}
	}

	if tuning.VisaDelayMs <= 0 || tuning.VisaCooldownMs <= 0 || tuning.ClearCartDelayMs <= 0 || tuning.ClearCartCooldownMs <= 0 {
		return fmt.Errorf("balance tuning skill delays and cooldowns must be positive")
	}
	if tuning.VisaDelayMs >= config.RoundLimitMs || tuning.ClearCartDelayMs >= config.RoundLimitMs {
		return fmt.Errorf("balance tuning skill delays must be shorter than hard round limit")
	}
	if tuning.ClearCartPickCount <= 0 || tuning.ClearCartPickCount > visibleCardCount {
		return fmt.Errorf("balance tuning clear cart pick count must fit one visible hand")
	}
	if tuning.NormalHighCardHandChance < 0 || tuning.NormalHighCardHandChance > 0.10 {
		return fmt.Errorf("balance tuning normal high-card chance must stay at or below ten percent")
	}
	if tuning.SpecialHighCardCount <= 0 || tuning.SpecialHighCardCount > visibleCardCount {
		return fmt.Errorf("balance tuning special high-card count must fit one visible hand")
	}
	if tuning.HighPriceThreshold <= 0 || tuning.HighPriceThreshold > config.InitialBalance {
		return fmt.Errorf("balance tuning high price threshold is invalid")
	}
	if tuning.EventBaseChance < 0 || tuning.EventBaseChance > 1 || tuning.EventRiskBonus < 0 || tuning.EventRiskBonus > 1 || tuning.EventMatchBonus < 0 || tuning.EventMatchBonus > 1 {
		return fmt.Errorf("balance tuning event chance values must be between zero and one")
	}

	if len(tuning.MultiplierRules) == 0 {
		return fmt.Errorf("balance tuning must include multiplier rules")
	}
	hasSingleMultiplier := false
	for _, rule := range tuning.MultiplierRules {
		if rule.ID == "" || rule.Label == "" {
			return fmt.Errorf("balance tuning multiplier rule has empty identity")
		}
		if rule.Multiplier <= 0 || rule.MinBalance < 0 || rule.MaxUnitPrice <= 0 || rule.MaxTotalPrice <= 0 || rule.Weight <= 0 {
			return fmt.Errorf("balance tuning multiplier rule has invalid numeric fields")
		}
		if rule.MinBalance > config.InitialBalance || rule.MaxUnitPrice > config.InitialBalance || rule.MaxTotalPrice > config.InitialBalance {
			return fmt.Errorf("balance tuning multiplier rule exceeds initial balance")
		}
		if rule.MaxUnitPrice > rule.MaxTotalPrice {
			return fmt.Errorf("balance tuning multiplier max unit price cannot exceed max total price")
		}
		if rule.Multiplier == 1 && rule.MinBalance == 0 {
			hasSingleMultiplier = true
		}
	}
	for _, lowerRule := range tuning.MultiplierRules {
		for _, higherRule := range tuning.MultiplierRules {
			if higherRule.Multiplier <= lowerRule.Multiplier {
				continue
			}

			/*
			 * 倍率卡的玩法承诺是“倍率越高，允许使用的单价和总额越低”。前端会逐张卡执行
			 * maxUnitPrice 和 maxTotalPrice，但如果后端调参时把 x20 的上限写得比 x3 还宽，
			 * 前端仍会照单执行，玩家就能用高倍率替代读卡寻找大额消费。这里在内容包边界
			 * 检查所有倍率组合，保证更高倍率不会拥有更宽的金额上限。
			 */
			if higherRule.MaxUnitPrice > lowerRule.MaxUnitPrice {
				return fmt.Errorf("balance tuning multiplier %q max unit price must not exceed lower multiplier %q", higherRule.ID, lowerRule.ID)
			}
			if higherRule.MaxTotalPrice > lowerRule.MaxTotalPrice {
				return fmt.Errorf("balance tuning multiplier %q max total price must not exceed lower multiplier %q", higherRule.ID, lowerRule.ID)
			}
		}
	}
	if !hasSingleMultiplier {
		return fmt.Errorf("balance tuning must include a zero-minimum x1 multiplier fallback")
	}

	return nil
}
