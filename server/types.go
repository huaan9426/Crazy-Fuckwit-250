package main

type gameConfig struct {
	InitialBalance int64         `json:"initialBalance"`
	RoundLimitMs   int64         `json:"roundLimitMs"`
	DefaultMode    string        `json:"defaultMode"`
	BalanceTuning  balanceTuning `json:"balanceTuning"`
	ContentVersion string        `json:"contentVersion"`
}

type multiplierRule struct {
	ID            string  `json:"id"`
	Label         string  `json:"label"`
	Multiplier    int64   `json:"multiplier"`
	MinBalance    int64   `json:"minBalance"`
	MaxUnitPrice  int64   `json:"maxUnitPrice"`
	MaxTotalPrice int64   `json:"maxTotalPrice"`
	Weight        float64 `json:"weight"`
}

type interestBand struct {
	MinBalance int64   `json:"minBalance"`
	Rate       float64 `json:"rate"`
}

type balanceTuning struct {
	StageCount               int64            `json:"stageCount"`
	StageDurationMs          int64            `json:"stageDurationMs"`
	TargetClearMs            int64            `json:"targetClearMs"`
	HandRefreshMs            int64            `json:"handRefreshMs"`
	SelectionSettleMs        int64            `json:"selectionSettleMs"`
	InterestStartDelayMs     int64            `json:"interestStartDelayMs"`
	InterestIntervalMs       int64            `json:"interestIntervalMs"`
	InterestRate             float64          `json:"interestRate"`
	InterestBands            []interestBand   `json:"interestBands"`
	VisaDelayMs              int64            `json:"visaDelayMs"`
	VisaCooldownMs           int64            `json:"visaCooldownMs"`
	ClearCartDelayMs         int64            `json:"clearCartDelayMs"`
	ClearCartCooldownMs      int64            `json:"clearCartCooldownMs"`
	ClearCartPickCount       int64            `json:"clearCartPickCount"`
	NormalHighCardHandChance float64          `json:"normalHighCardHandChance"`
	SpecialHighCardCount     int64            `json:"specialHighCardCount"`
	HighPriceThreshold       int64            `json:"highPriceThreshold"`
	EventBaseChance          float64          `json:"eventBaseChance"`
	EventRiskBonus           float64          `json:"eventRiskBonus"`
	EventMatchBonus          float64          `json:"eventMatchBonus"`
	MultiplierRules          []multiplierRule `json:"multiplierRules"`
}

type item struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	SceneID    *string  `json:"sceneId"`
	Price      int64    `json:"price"`
	Tier       string   `json:"tier"`
	MaxBuy     *int64   `json:"maxBuy"`
	Batchable  bool     `json:"batchable"`
	Weight     int64    `json:"weight"`
	MinBalance int64    `json:"minBalance"`
	Modes      []string `json:"modes"`
	Tags       []string `json:"tags"`
	Flavor     string   `json:"flavor"`
}

type scene struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	EntryCost   int64    `json:"entryCost"`
	DurationSec int64    `json:"durationSec"`
	MinBalance  int64    `json:"minBalance"`
	Rarity      string   `json:"rarity"`
	RiskLevel   int64    `json:"riskLevel"`
	ItemTags    []string `json:"itemTags"`
	EventTags   []string `json:"eventTags"`
	Modes       []string `json:"modes"`
}

type gameEvent struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Delta         *int64   `json:"delta,omitempty"`
	Probability   float64  `json:"probability"`
	CooldownSec   int64    `json:"cooldownSec"`
	Tags          []string `json:"tags"`
	Modes         []string `json:"modes"`
	SettlementTag string   `json:"settlementTag"`
}

type terminalEvent struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Probability   float64  `json:"probability"`
	MinElapsedMs  int64    `json:"minElapsedMs"`
	MaxBalance    *int64   `json:"maxBalance"`
	MinRiskLevel  int64    `json:"minRiskLevel"`
	BalanceEffect string   `json:"balanceEffect"`
	Tags          []string `json:"tags"`
	Modes         []string `json:"modes"`
	SettlementTag string   `json:"settlementTag"`
}

type statusEffect struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	DurationSec           int64    `json:"durationSec"`
	ItemRefreshMultiplier float64  `json:"itemRefreshMultiplier"`
	HighPriceMultiplier   float64  `json:"highPriceMultiplier"`
	EventMultiplier       float64  `json:"eventMultiplier"`
	Tags                  []string `json:"tags"`
	Description           string   `json:"description"`
}

type audioTrack struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Mood      string `json:"mood"`
	Src       string `json:"src"`
	License   string `json:"license"`
	SourceURL string `json:"sourceUrl"`
}

type bootstrapResponse struct {
	Config      gameConfig      `json:"config"`
	Items       []item          `json:"items"`
	Scenes      []scene         `json:"scenes"`
	Events      []gameEvent     `json:"events"`
	Endings     []terminalEvent `json:"endings"`
	Statuses    []statusEffect  `json:"statuses"`
	AudioTracks []audioTrack    `json:"audioTracks"`
}

type reserveRequest struct {
	Username         string `json:"username"`
	ReservationToken string `json:"reservationToken,omitempty"`
}

type reserveResponse struct {
	Username         string `json:"username"`
	Reserved         bool   `json:"reserved"`
	ReservationToken string `json:"reservationToken,omitempty"`
	Message          string `json:"message,omitempty"`
}

type runSubmission struct {
	Username         string `json:"username"`
	DurationMs       int64  `json:"durationMs"`
	MaxSingleSpend   int64  `json:"maxSingleSpend"`
	FinalBalance     int64  `json:"finalBalance"`
	TotalSpent       int64  `json:"totalSpent"`
	TotalIncome      int64  `json:"totalIncome"`
	EndedBy          string `json:"endedBy"`
	ChaosSeed        string `json:"chaosSeed"`
	ContentVersion   string `json:"contentVersion,omitempty"`
	ReservationToken string `json:"reservationToken,omitempty"`
	EndingID         string `json:"endingId,omitempty"`
	EndingTitle      string `json:"endingTitle,omitempty"`
	EndingDetail     string `json:"endingDetail,omitempty"`
}

type leaderboardEntry struct {
	Rank           int    `json:"rank"`
	Username       string `json:"username"`
	DurationMs     int64  `json:"durationMs"`
	MaxSingleSpend int64  `json:"maxSingleSpend"`
	FinalBalance   int64  `json:"-"`
	EndedBy        string `json:"-"`
}

type runResult struct {
	Accepted bool             `json:"accepted"`
	Entry    leaderboardEntry `json:"entry"`
	Message  string           `json:"message,omitempty"`
}

type apiErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
