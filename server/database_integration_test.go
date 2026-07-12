package main

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

var integrationUsernameCounter uint64

func openIntegrationDatabase(t *testing.T) *appState {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL != "" {
		t.Setenv("DATABASE_URL", databaseURL)
	} else if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Skip("set DATABASE_URL or TEST_DATABASE_URL to run PostgreSQL integration test")
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

func integrationUsername() string {
	return "集成" + strconv.FormatUint(atomic.AddUint64(&integrationUsernameCounter, 1), 36)
}

func cleanupIntegrationUsername(t *testing.T, state *appState, username string) {
	t.Helper()

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
		defer cancel()
		if _, err := state.db.ExecContext(ctx, `DELETE FROM runs WHERE username = $1`, username); err != nil {
			t.Fatalf("cleanup integration run: %v", err)
		}
		if _, err := state.db.ExecContext(ctx, `DELETE FROM usernames WHERE username = $1`, username); err != nil {
			t.Fatalf("cleanup integration username: %v", err)
		}
	}

	cleanup()
	t.Cleanup(cleanup)
}

func TestDatabaseBootstrapAndRunLifecycle(t *testing.T) {
	state := openIntegrationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), databaseRequestTimeout)
	defer cancel()

	/*
	 * 这个集成测试只保留一条完整主链路。它先读取真实 PostgreSQL 内容包，证明 schema、
	 * seed 和 Go 扫描字段仍然连通；随后预约用户名、提交一局成绩并读取排行榜，证明最重要的
	 * 写入事务和排序查询没有断。详细的每列 CHECK 约束由 schema.sql 自己表达，不再为每个
	 * 约束复制一组几十行测试，避免测试代码比业务代码更臃肿。
	 */
	bootstrap, err := state.loadBootstrapFromDatabase(ctx)
	if err != nil {
		t.Fatalf("load bootstrap from database: %v", err)
	}
	if len(bootstrap.Items) < minimumBootstrapItems || len(bootstrap.Scenes) < minimumBootstrapScenes {
		t.Fatalf("bootstrap content is incomplete: items=%d scenes=%d", len(bootstrap.Items), len(bootstrap.Scenes))
	}
	if !isGeneratedContentVersion(bootstrap.Config.ContentVersion) {
		t.Fatalf("bootstrap content version = %q", bootstrap.Config.ContentVersion)
	}

	/*
	 * auto-card-* 是早期为了补齐价位覆盖生成的稳定 id。现在这些 id 仍然保留，避免前端、
	 * 成绩内容版本和后续美术映射失去连接；但展示名称已经改成 220 个具体消费项目。这里同时
	 * 检查数量、长度、唯一性和旧后缀，是为了让 seed 以后调整金额时不会顺手退回“基础款、
	 * 周末款、离谱款”的拼接名称。十二个汉字的上限来自当前移动端三列卡牌的两行标题空间。
	 */
	legacyTemplateSuffixes := []string{
		"·基础款", "·加急款", "·升级款", "·押金款", "·套餐款", "·周末款",
		"·误操作款", "·保价失败款", "·高峰款", "·隐藏成本款", "·离谱款",
	}
	itemNames := make(map[string]string, len(bootstrap.Items))
	templateItemCount := 0
	for _, candidate := range bootstrap.Items {
		if previousID, exists := itemNames[candidate.Name]; exists {
			t.Fatalf("duplicate item name %q for ids %q and %q", candidate.Name, previousID, candidate.ID)
		}
		itemNames[candidate.Name] = candidate.ID

		if !strings.HasPrefix(candidate.ID, "auto-card-") {
			continue
		}
		templateItemCount++
		if len([]rune(candidate.Name)) > 12 {
			t.Fatalf("template item %q name is too long for the mobile card: %q", candidate.ID, candidate.Name)
		}
		for _, suffix := range legacyTemplateSuffixes {
			if strings.Contains(candidate.Name, suffix) {
				t.Fatalf("template item %q still uses legacy suffix in name %q", candidate.ID, candidate.Name)
			}
		}
	}
	if templateItemCount != 220 {
		t.Fatalf("template item count = %d, want 220", templateItemCount)
	}

	username := integrationUsername()
	cleanupIntegrationUsername(t, state, username)
	reserved, reservationToken, err := state.reserveUsernameInDatabase(ctx, username, "")
	if err != nil || !reserved || reservationToken == "" {
		t.Fatalf("reserve username: reserved=%v token=%q err=%v", reserved, reservationToken, err)
	}

	run := validRunForTest(username)
	run.ContentVersion = bootstrap.Config.ContentVersion
	run.ReservationToken = reservationToken
	entry, err := state.submitRunToDatabase(ctx, run)
	if err != nil {
		t.Fatalf("submit run: %v", err)
	}
	if entry.Username != username || entry.Rank < 1 {
		t.Fatalf("submitted entry = %+v", entry)
	}

	if _, err := state.submitRunToDatabase(ctx, run); !errors.Is(err, errDuplicateRun) {
		t.Fatalf("duplicate run error = %v, want %v", err, errDuplicateRun)
	}

	entries, err := state.leaderboardFromDatabase(ctx, leaderboardLimit, bootstrap.Config.ContentVersion)
	if err != nil {
		t.Fatalf("load leaderboard: %v", err)
	}
	found := false
	for _, candidate := range entries {
		if candidate.Username == username {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("submitted username %q was not found in leaderboard", username)
	}
}
