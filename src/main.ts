import "./styles.css";

const items = [
  {
    key: "1",
    name: "便利店早餐",
    price: "¥18",
    tag: "基础消费",
    scene: "日常小额",
    rarity: "N",
    risk: "可批量 x100",
    odds: "常见",
    heat: 92
  },
  {
    key: "2",
    name: "普通外卖",
    price: "¥32",
    tag: "基础消费",
    scene: "工作日午晚饭",
    rarity: "N",
    risk: "高频消耗",
    odds: "常见",
    heat: 89
  },
  {
    key: "3",
    name: "奶茶全员请客",
    price: "¥486",
    tag: "社交小额",
    scene: "办公室拼单",
    rarity: "N",
    risk: "容易连点",
    odds: "常见",
    heat: 76
  },
  {
    key: "4",
    name: "打车跨城加价",
    price: "¥318",
    tag: "交通",
    scene: "临时赶路",
    rarity: "R",
    risk: "高峰溢价",
    odds: "较常见",
    heat: 67
  },
  {
    key: "5",
    name: "演唱会前排票",
    price: "¥2,880",
    tag: "娱乐",
    scene: "追星冲动",
    rarity: "R",
    risk: "酒店机票连锁",
    odds: "较少",
    heat: 58
  },
  {
    key: "6",
    name: "手机碎屏换新",
    price: "¥6,999",
    tag: "数码意外",
    scene: "日常灾难",
    rarity: "SR",
    risk: "数据恢复加价",
    odds: "稀有",
    heat: 48
  },
  {
    key: "7",
    name: "宠物急诊检查",
    price: "¥4,600",
    tag: "宠物",
    scene: "突发账单",
    rarity: "SR",
    risk: "住院连锁",
    odds: "稀有",
    heat: 44
  },
  {
    key: "8",
    name: "装修临时增项",
    price: "¥18,800",
    tag: "大件现实",
    scene: "装修翻车",
    rarity: "SSR",
    risk: "返工连锁",
    odds: "极少",
    heat: 31
  },
  {
    key: "9",
    name: "拍卖误举牌",
    price: "¥68,000",
    tag: "高端误操作",
    scene: "拍卖预展",
    rarity: "SSR",
    risk: "不可撤回",
    odds: "极低",
    heat: 14
  }
];

const scenes = [
  {
    name: "便利店清空",
    type: "基础场景",
    cost: "¥1 - ¥99",
    chance: "高频",
    detail: "找零、泡面、咖啡、纸巾、电池，负责让 250 万显得真的经花。"
  },
  {
    name: "工作日循环",
    type: "日常场景",
    cost: "¥18 - ¥680",
    chance: "高频",
    detail: "早餐、外卖、打车、会员续费、人情小红包，密集但不刺激。"
  },
  {
    name: "装修翻车",
    type: "现实重击",
    cost: "¥3,000 - ¥80,000",
    chance: "低频",
    detail: "增项、漏水、返工、家电、清洁，一次把日常节奏打断。"
  },
  {
    name: "面具舞会",
    type: "富人幻觉",
    cost: "¥8,000 - ¥120,000",
    chance: "极低",
    detail: "礼服、专车、慈善捐款、误入拍卖，是稀有高压副本。"
  }
];

const events = [
  { type: "status", time: "03:11", title: "状态变化", text: "心悸触发，医疗事件概率 +45%" },
  { type: "income", time: "03:12", title: "反向进账", text: "退税到账 +¥7,300，清空进度被拖回" },
  { type: "danger", time: "03:13", title: "追加扣款", text: "游艇清洁费追加 -¥18,000" },
  { type: "warn", time: "03:14", title: "情绪失控", text: "生气状态，高价商品刷新暴涨" },
  { type: "danger", time: "03:16", title: "误操作", text: "拍卖师落槌，误举牌已入账" },
  { type: "income", time: "03:17", title: "赔付", text: "酒店超售赔付 +¥2,000" }
];

const ranks = [
  ["01", "冷静不了一点", "02:41", "¥188,000"],
  ["02", "今晚就花完", "03:09", "¥162,000"],
  ["03", "余额恐惧症", "03:57", "¥128,000"],
  ["04", "退款杀我", "04:22", "¥96,000"]
];

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <main class="game-shell">
    <section class="topbar">
      <div>
        <p class="eyebrow">混沌人生 · 所有事件包开启</p>
        <h1>250 万清空挑战</h1>
      </div>
      <div class="timer">
        <span>用时</span>
        <strong>03:17.42</strong>
      </div>
    </section>

    <section class="status-grid" aria-label="游戏状态">
      <article class="stat-card balance-card">
        <span>当前余额</span>
        <strong>¥347,582</strong>
        <small>刚被退税回血，清空失败风险上升</small>
      </article>
      <article class="stat-card">
        <span>已花出去</span>
        <strong>¥2,287,940</strong>
        <small>总支出已超过本金</small>
      </article>
      <article class="stat-card income-card">
        <span>意外进账</span>
        <strong>+¥135,522</strong>
        <small>退款、赔付、中奖正在捣乱</small>
      </article>
      <article class="stat-card pressure-card">
        <span>混沌压力</span>
        <strong>92%</strong>
        <small>终局事件已进入概率池</small>
      </article>
    </section>

    <section class="game-board">
      <div class="live-panel">
        <div class="panel-head">
          <div>
            <p class="section-kicker">当前货架</p>
            <h2>基础消费正在快速消耗</h2>
          </div>
          <div class="multiplier">
            <button type="button">x1</button>
            <button type="button" class="active">x10</button>
            <button type="button">x100</button>
            <button type="button">买到爆</button>
          </div>
        </div>
        <div class="items-grid">
          ${items
            .map(
              (item) => `
                <button class="item-card rarity-${item.rarity.toLowerCase()}" type="button">
                  <span class="hotbar">${item.key}</span>
                  <span class="rarity">${item.rarity}</span>
                  <span class="odds">${item.odds}</span>
                  <span class="item-tag">${item.tag}</span>
                  <strong>${item.name}</strong>
                  <em>${item.price}</em>
                  <span class="item-meta">
                    <b>${item.scene}</b>
                    <i>${item.risk}</i>
                  </span>
                  <span class="heat" aria-label="刷新热度"><i style="width: ${item.heat}%"></i></span>
                </button>
              `
            )
            .join("")}
        </div>
      </div>

      <aside class="chaos-panel">
        <div class="scene-list-card">
          <div class="panel-head compact">
            <div>
              <p class="section-kicker">场景卡池</p>
              <h2>不是所有高价都随便出现</h2>
            </div>
          </div>
          <div class="scene-list">
            ${scenes
              .map(
                (scene) => `
                  <article class="mini-scene">
                    <span>${scene.type}</span>
                    <strong>${scene.name}</strong>
                    <b>${scene.cost}</b>
                    <small>${scene.chance}</small>
                    <p>${scene.detail}</p>
                  </article>
                `
              )
              .join("")}
          </div>
        </div>

        <div class="scene-card">
          <p class="section-kicker">正在发生的场景</p>
          <h2>工作日循环 · 限时 00:23</h2>
          <p>当前不是富人副本，而是普通人的高频消耗池。小额商品更多，但会被批量购买放大。</p>
          <div class="scene-effects">
            <span>外卖连点</span>
            <span>打车溢价</span>
            <span>会员续费</span>
            <span>小额红包</span>
          </div>
          <div class="scene-meter">
            <span style="width: 64%"></span>
          </div>
          <small>场景热度 64%，高价稀有卡仍可能突然插入。</small>
        </div>

        <div class="event-feed">
          <div class="panel-head compact">
            <div>
              <p class="section-kicker">实时事件流水</p>
              <h2>刚刚发生了什么</h2>
            </div>
            <span class="blink">LIVE</span>
          </div>
          ${events
            .map(
              (event) => `
                <p class="event ${event.type}">
                  <span class="event-dot"></span>
                  <time>${event.time}</time>
                  <b>${event.title}</b>
                  <em>${event.text}</em>
                </p>
              `
            )
            .join("")}
        </div>
      </aside>
    </section>

    <section class="bottom-grid">
      <article class="report-card">
        <p class="section-kicker">战报预览</p>
        <h2>最烦人返钱：彩票刮中 +¥100,000</h2>
        <p>最大单笔消费暂为游艇派对押金。若 60 秒内余额未清零，终局包权重继续上升。</p>
      </article>
      <article class="leaderboard-card">
        <div class="panel-head compact">
          <h2>排行榜</h2>
          <span>只看四项</span>
        </div>
        <table>
          <thead>
            <tr>
              <th>名次</th>
              <th>用户名</th>
              <th>用时</th>
              <th>单笔最高</th>
            </tr>
          </thead>
          <tbody>
            ${ranks
              .map(
                ([rank, name, time, spend]) => `
                  <tr>
                    <td>${rank}</td>
                    <td>${name}</td>
                    <td>${time}</td>
                    <td>${spend}</td>
                  </tr>
                `
              )
              .join("")}
          </tbody>
        </table>
      </article>
    </section>
  </main>
`;
