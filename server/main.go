package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxJSONBodyBytes       = 64 << 10
	defaultAPIPort         = "3001"
	defaultWebOrigin       = "http://localhost:5173"
	leaderboardLimit       = 20
	minUsernameRunes       = 2
	maxUsernameRunes       = 16
	forbiddenUsernameChars = "<>{}[]/\\|"
	minAcceptedDurationMs  = 1_000
	maxAcceptedRunMoney    = defaultInitialBalance * 1_000
	usernameReservationTTL = 30 * time.Minute
	databaseHealthTimeout  = 800 * time.Millisecond
)

var errDuplicateRun = errors.New("username already has a submitted run")
var errUsernameReserved = errors.New("username is reserved by another active session")

// appState 保存服务运行时需要共享的依赖。db 不为空时，内容、用户名、成绩和排行榜都
// 走 PostgreSQL；db 为空时，服务退回到内存用户名和内存排行榜，方便没有数据库的本地
// 前端开发。mu 只保护内存兜底路径，数据库路径依赖 PostgreSQL 的唯一索引和事务保证
// 并发正确性，避免 Go 代码里一边拿锁一边等待数据库造成不必要的阻塞。
type appState struct {
	db        *sql.DB
	mu        sync.Mutex
	usernames map[string]usernameReservation
	runs      []leaderboardEntry
}

type usernameReservation struct {
	Token         string
	ReservedUntil time.Time
}

func main() {
	db, err := openConfiguredDatabase()
	if err != nil {
		log.Fatalf("PostgreSQL configured but unavailable: %v", err)
	}
	if db != nil {
		defer db.Close()
		log.Printf("PostgreSQL enabled for content, usernames, runs, and leaderboard")
	} else {
		log.Printf("PostgreSQL not configured, using in-memory store for local development")
	}

	state := &appState{
		db:        db,
		usernames: make(map[string]usernameReservation),
		runs:      seedLeaderboard(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", state.handleHealth)
	mux.HandleFunc("GET /api/content/bootstrap", state.handleBootstrap)
	mux.HandleFunc("POST /api/users/reserve", state.handleReserveUser)
	mux.HandleFunc("POST /api/runs", state.handleSubmitRun)
	mux.HandleFunc("GET /api/leaderboard", state.handleLeaderboard)

	port := envOrDefault("API_PORT", defaultAPIPort)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Go API listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func seedLeaderboard() []leaderboardEntry {
	return []leaderboardEntry{
		{Rank: 1, Username: "冷静不了一点", DurationMs: 161_000, MaxSingleSpend: 188_000, FinalBalance: 0, EndedBy: "balance_zero"},
		{Rank: 2, Username: "今晚就花完", DurationMs: 189_000, MaxSingleSpend: 162_000, FinalBalance: 0, EndedBy: "balance_zero"},
		{Rank: 3, Username: "退款杀我", DurationMs: 237_000, MaxSingleSpend: 128_000, FinalBalance: 0, EndedBy: "balance_zero"},
	}
}

func (state *appState) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !state.hasDatabase() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "fallback"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), databaseHealthTimeout)
	defer cancel()

	/*
	 * /healthz 是前端恢复探活和人工联调最先看的接口。只要 state.db 不为空就返回
	 * database:"online" 会掩盖一种真实故障：Go 进程还活着，但 PostgreSQL 已经断开或
	 * 不可响应。这里用很短的 PingContext 证明数据库当前可用；失败时返回 503，让前端
	 * 不会把数据库故障误显示成“Go API 已接通”。
	 */
	if err := state.db.PingContext(ctx); err != nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "database_unavailable", "数据库探活失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "online"})
}

func (state *appState) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), databaseRequestTimeout)
	defer cancel()

	bootstrap, err := state.loadBootstrap(ctx)
	if err != nil {
		log.Printf("load bootstrap from database failed: %v", err)
		writeErrorJSON(w, http.StatusInternalServerError, "database_error", "内容包读取数据库失败")
		return
	}

	writeJSON(w, http.StatusOK, bootstrap)
}

func (state *appState) handleReserveUser(w http.ResponseWriter, r *http.Request) {
	var payload reserveRequest
	if !decodeJSON(w, r, &payload) {
		return
	}

	username := normalizeUsername(payload.Username)
	if !validUsername(username) {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_username", "用户名需要 2 到 16 个字，且不能包含尖括号、斜杠等特殊字符")
		return
	}

	if state.hasDatabase() {
		ctx, cancel := context.WithTimeout(r.Context(), databaseRequestTimeout)
		defer cancel()

		reserved, reservationToken, err := state.reserveUsernameInDatabase(ctx, username, payload.ReservationToken)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "database_error", "用户名占用写入数据库失败")
			return
		}

		if !reserved {
			writeJSON(w, http.StatusOK, reserveResponse{Username: username, Reserved: false, Message: "用户名已被占用"})
			return
		}

		writeJSON(w, http.StatusOK, reserveResponse{Username: username, Reserved: true, ReservationToken: reservationToken})
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	reserved, reservationToken, err := state.reserveUsernameInMemory(username, payload.ReservationToken, time.Now())
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "reservation_error", "用户名预约生成失败")
		return
	}
	if !reserved {
		writeJSON(w, http.StatusOK, reserveResponse{Username: username, Reserved: false, Message: "用户名已被占用"})
		return
	}

	writeJSON(w, http.StatusOK, reserveResponse{Username: username, Reserved: true, ReservationToken: reservationToken})
}

func (state *appState) handleSubmitRun(w http.ResponseWriter, r *http.Request) {
	var payload runSubmission
	if !decodeJSON(w, r, &payload) {
		return
	}

	payload.Username = normalizeUsername(payload.Username)
	if err := validateRun(payload); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_run", err.Error())
		return
	}

	if state.hasDatabase() {
		ctx, cancel := context.WithTimeout(r.Context(), databaseRequestTimeout)
		defer cancel()

		entry, err := state.submitRunToDatabase(ctx, payload)
		if err != nil {
			if errors.Is(err, errDuplicateRun) || errors.Is(err, errUsernameReserved) {
				writeErrorJSON(w, http.StatusConflict, "run_not_accepted", err.Error())
				return
			}

			log.Printf("submit run to database failed: %v", err)
			writeErrorJSON(w, http.StatusInternalServerError, "database_error", "成绩写入数据库失败")
			return
		}

		writeJSON(w, http.StatusCreated, runResult{Accepted: true, Entry: entry})
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	for _, existing := range state.runs {
		if existing.Username == payload.Username {
			writeErrorJSON(w, http.StatusConflict, "run_not_accepted", "username already has a submitted run")
			return
		}
	}
	if !state.canSubmitReservedUsernameLocked(payload.Username, payload.ReservationToken, time.Now()) {
		writeErrorJSON(w, http.StatusConflict, "run_not_accepted", errUsernameReserved.Error())
		return
	}

	// 数据库路径在 submitRunToDatabase 里会先把用户名写入 usernames 表，再写入 runs 表。
	// 内存兜底也必须做同一件事：有些调用方可能因为网络重试、开发调试或旧前端流程，
	// 没有先请求 /api/users/reserve，而是直接提交 /api/runs。如果这里不把用户名放进
	// state.usernames，后续再预约同一个名字时会被错误放行，导致无数据库开发环境看到
	// 一套和 PostgreSQL 不一样的业务规则。
	if state.usernames == nil {
		state.usernames = make(map[string]usernameReservation)
	}
	state.usernames[payload.Username] = usernameReservation{}

	entry := leaderboardEntry{
		Username:       payload.Username,
		DurationMs:     payload.DurationMs,
		MaxSingleSpend: payload.MaxSingleSpend,
		FinalBalance:   payload.FinalBalance,
		EndedBy:        payload.EndedBy,
	}
	state.runs = append(state.runs, entry)
	state.rankRunsLocked()

	// 这里重新遍历一次排行榜，是为了把排序后真实名次写回给前端。前端提交时只知道
	// 自己这一局的用户名、用时和最大单笔消费，不知道插入排行榜后会排第几。后端排序后
	// 找到同一条成绩再返回，前端就可以继续用同一个 response shape 展示结果。
	for _, ranked := range state.runs {
		if ranked.Username == payload.Username && ranked.DurationMs == payload.DurationMs {
			writeJSON(w, http.StatusCreated, runResult{Accepted: true, Entry: ranked})
			return
		}
	}

	writeJSON(w, http.StatusCreated, runResult{Accepted: true, Entry: entry})
}

func (state *appState) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if state.hasDatabase() {
		ctx, cancel := context.WithTimeout(r.Context(), databaseRequestTimeout)
		defer cancel()

		entries, err := state.leaderboardFromDatabase(ctx, leaderboardLimit, optionalContentVersion(r.URL.Query().Get("contentVersion")))
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "database_error", "排行榜读取数据库失败")
			return
		}

		writeJSON(w, http.StatusOK, entries)
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.rankRunsLocked()
	limit := leaderboardLimit
	if len(state.runs) < limit {
		limit = len(state.runs)
	}
	writeJSON(w, http.StatusOK, state.runs[:limit])
}

func (state *appState) rankRunsLocked() {
	// 这个函数名字里的 Locked 是提醒调用者：进入这里之前必须已经拿到 state.mu。
	// 排行榜排序会直接重排 state.runs 这个切片，如果不加锁，另一个请求可能刚好也在
	// append 新成绩或读取排行榜，结果就会出现顺序混乱。排序规则必须和 PostgreSQL 查询保持
	// 一致：最终余额归零的成绩优先；同为归零或同为其他结局时，用时越短越靠前；用时相同则
	// 单笔最高消费越高越靠前。FinalBalance 和 EndedBy 都用 json:"-" 保存在内存里，不会扩大
	// 排行榜 API 展示字段。
	sort.SliceStable(state.runs, func(i int, j int) bool {
		firstCleared := state.runs[i].FinalBalance == 0
		secondCleared := state.runs[j].FinalBalance == 0
		if firstCleared != secondCleared {
			return firstCleared
		}

		if state.runs[i].DurationMs == state.runs[j].DurationMs {
			return state.runs[i].MaxSingleSpend > state.runs[j].MaxSingleSpend
		}
		return state.runs[i].DurationMs < state.runs[j].DurationMs
	})

	for index := range state.runs {
		state.runs[index].Rank = index + 1
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	// JSON 是前端和后端之间传结构化数据的文本格式。浏览器提交用户名或成绩时，
	// body 里不是 Go 结构体，而是一段类似 {"username":"今晚就花完"} 的 JSON 文本。
	// decoder 会把这段文本填进 destination 指向的 Go 结构体。这里限制 body 大小，
	// 是为了防止客户端传一个异常大的请求拖垮内存；DisallowUnknownFields 则要求前端
	// 只能提交后端认识的字段，避免拼写错误或多余字段悄悄被忽略。第一次 Decode 成功
	// 后还要再读一次，是为了确认请求体后面没有第二个 JSON 对象或垃圾内容；否则客户端
	// 传 {"username":"a"}{"username":"b"} 这类拼接数据时，后端可能只处理前半段。
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_json", "请求体必须是后端认识的 JSON 字段")
		return false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_json", "请求体只能包含一个完整 JSON 对象")
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json failed: %v", err)
	}
}

func writeErrorJSON(w http.ResponseWriter, status int, code string, message string) {
	// 失败响应也用 JSON，而不是 http.Error 默认生成的纯文本。这样前端可以稳定读取
	// code 和 message：code 给程序判断错误类型，message 给界面展示或调试。比如用户
	// 名非法、JSON 字段写错、成绩字段不合理，前端都能知道这是服务端明确拒绝的请求，
	// 不能把它误判成“后端离线，然后走本地兜底”。
	writeJSON(w, status, apiErrorResponse{Code: code, Message: message})
}

func normalizeUsername(username string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(username)), " ")
}

func validUsername(username string) bool {
	if len([]rune(username)) < minUsernameRunes || len([]rune(username)) > maxUsernameRunes {
		return false
	}
	if username != normalizeUsername(username) {
		return false
	}

	return !strings.ContainsAny(username, forbiddenUsernameChars)
}

func (state *appState) reserveUsernameInMemory(username string, providedToken string, now time.Time) (bool, string, error) {
	for _, existing := range state.runs {
		if existing.Username == username {
			return false, "", nil
		}
	}

	if state.usernames == nil {
		state.usernames = make(map[string]usernameReservation)
	}

	if reservation, exists := state.usernames[username]; exists {
		if reservation.Token != "" && now.Before(reservation.ReservedUntil) {
			if providedToken != "" && reservation.Token == providedToken {
				reservation.ReservedUntil = now.Add(usernameReservationTTL)
				state.usernames[username] = reservation
				return true, reservation.Token, nil
			}
			return false, "", nil
		}
	}

	token, err := newReservationToken()
	if err != nil {
		return false, "", err
	}
	state.usernames[username] = usernameReservation{
		Token:         token,
		ReservedUntil: now.Add(usernameReservationTTL),
	}
	return true, token, nil
}

func (state *appState) canSubmitReservedUsernameLocked(username string, providedToken string, now time.Time) bool {
	reservation, exists := state.usernames[username]
	if !exists || reservation.Token == "" || !now.Before(reservation.ReservedUntil) {
		return true
	}

	return providedToken != "" && providedToken == reservation.Token
}

func newReservationToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func validateRun(run runSubmission) error {
	// 这不是完整反作弊，只是第一层基础校验。前端运行在玩家浏览器里，任何请求都可能被
	// 手工伪造，所以后端至少要拒绝明显不合理的成绩，例如用户名非法、用时过短、金额为负、
	// 结束原因不是约定值。金额字段之间也要能对上账：当前首版固定初始余额为 2,500,000，
	// 前端每次扣款会累加 totalSpent，每次返钱和利息会累加 totalIncome，最后提交的
	// finalBalance 应该等于“初始余额 + 收入 - 支出”。这仍然不是完整反作弊，因为玩家
	// 仍可以伪造一整套自洽的数据；但它能挡住明显矛盾的排行榜成绩，避免数据库里出现
	// “标记已清空但还剩钱”或“最大单笔大于总支出”这类后续无法解释的记录。
	if !validUsername(run.Username) {
		return errors.New("invalid username")
	}
	if run.DurationMs < minAcceptedDurationMs {
		return errors.New("duration too short")
	}
	if run.DurationMs > defaultRoundLimitMs {
		return errors.New("duration exceeds hard round limit")
	}
	if run.MaxSingleSpend < 0 || run.FinalBalance < 0 || run.TotalSpent < 0 || run.TotalIncome < 0 {
		return errors.New("money fields must be non-negative")
	}
	if run.MaxSingleSpend > maxAcceptedRunMoney || run.FinalBalance > maxAcceptedRunMoney || run.TotalSpent > maxAcceptedRunMoney || run.TotalIncome > maxAcceptedRunMoney {
		return errors.New("money fields exceed accepted range")
	}
	if run.MaxSingleSpend > run.TotalSpent {
		return errors.New("max single spend cannot exceed total spent")
	}
	if run.EndedBy != "balance_zero" && run.EndedBy != "timeout" && run.EndedBy != "manual" && run.EndedBy != "terminal_event" {
		return errors.New("invalid endedBy")
	}

	expectedFinalBalance := defaultInitialBalance + run.TotalIncome - run.TotalSpent
	if expectedFinalBalance != run.FinalBalance {
		return errors.New("money fields do not balance")
	}
	if run.EndedBy == "balance_zero" && run.FinalBalance != 0 {
		return errors.New("balance_zero run must end with zero balance")
	}
	if run.EndedBy == "timeout" && run.FinalBalance == 0 {
		return errors.New("timeout run must keep remaining balance")
	}
	/*
	 * manual 是为了将来保留“玩家主动放弃、管理员中止、调试工具停表”这类外部结束入口。
	 * 它可以提交一局没有清空的摘要，方便本地战报或排错；但如果余额已经等于 0，这局应该
	 * 明确归类为 balance_zero 或 terminal_event。否则手工请求可以用 manual 伪造一条
	 * 最快清零成绩，绕开前端实际的结束原因，排行榜也无法解释这条记录为什么算通关。
	 */
	if run.EndedBy == "manual" && run.FinalBalance == 0 {
		return errors.New("manual run must keep remaining balance")
	}
	/*
	 * timeout 在这个游戏里不是普通失败理由，而是“11 分钟硬结算线到了”。前端的阶段系统
	 * 只会切换消费压力，第 12 段结束不会自动停表；真正能产生 timeout 的路径只有硬结算
	 * 时钟。因此后端也必须要求 timeout 的 durationMs 等于硬结算时长，否则几秒钟的伪造
	 * 请求就能写成“时间到但还剩钱”，排行榜和结算日志都会变成无法解释的数据。
	 */
	if run.EndedBy == "timeout" && run.DurationMs != defaultRoundLimitMs {
		return errors.New("timeout run must end at hard round limit")
	}
	if strings.TrimSpace(run.ChaosSeed) == "" {
		return errors.New("missing chaosSeed")
	}
	if run.EndedBy == "terminal_event" {
		if strings.TrimSpace(run.EndingID) == "" {
			return errors.New("missing terminal event id")
		}
		if strings.TrimSpace(run.EndingTitle) == "" {
			return errors.New("missing terminal event title")
		}
		if strings.TrimSpace(run.EndingDetail) == "" {
			return errors.New("missing terminal event detail")
		}
	} else if strings.TrimSpace(run.EndingID) != "" || strings.TrimSpace(run.EndingTitle) != "" || strings.TrimSpace(run.EndingDetail) != "" {
		/*
		 * endingId、endingTitle 和 endingDetail 是“终局事件”的解释字段，不是普通成绩备注。
		 * 余额归零、硬结算和手动结束都有自己的 endedBy 含义；如果这些非终局成绩还能夹带
		 * 终局文案，数据库里就会出现“看起来是正常清空，但又带着特殊终局说明”的混合记录。
		 * 前端当前不会主动这样提交，但后端边界必须挡住手工请求、测试脚本或后台工具写入这类
		 * 数据，保证排行榜摘要和结算日志能够按同一套语义解释。
		 */
		return errors.New("non-terminal run cannot include terminal event fields")
	}

	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	origin := envOrDefault("WEB_ORIGIN", defaultWebOrigin)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
