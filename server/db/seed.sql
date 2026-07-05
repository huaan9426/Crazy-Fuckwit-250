-- 首版没有后台内容管理页面，当前 chaos-life 模式的内容事实源就是这份 seed.sql。
-- 启动时先把旧的当前模式内容标为 inactive，再由下面仍然存在的 seed 行重新激活。
-- 这样以后删除或重命名一张卡、一个场景或一个事件时，旧数据库里残留的行不会继续进入前端抽卡和事件算法。
UPDATE content_items SET active = false WHERE modes ? 'chaos-life';
UPDATE content_scenes SET active = false WHERE modes ? 'chaos-life';
UPDATE content_events SET active = false WHERE modes ? 'chaos-life';
UPDATE content_endings SET active = false WHERE modes ? 'chaos-life';
UPDATE content_statuses SET active = false;
UPDATE audio_tracks SET active = false;

INSERT INTO content_scenes (
  id, name, entry_cost, duration_sec, min_balance, rarity, risk_level, item_tags, event_tags, sort_order
) VALUES
  ('daily-loop', '工作日循环', 0, 35, 0, 'common', 2, '["daily","traffic","social","platform"]'::jsonb, '["refund","surge","fee"]'::jsonb, 10),
  ('late-night-cart', '深夜购物车', 0, 35, 0, 'common', 3, '["snack","platform","impulse"]'::jsonb, '["coupon","refund"]'::jsonb, 20),
  ('repair-week', '维修翻车周', 0, 35, 5000, 'rare', 4, '["repair","digital","home"]'::jsonb, '["damage","fee"]'::jsonb, 30),
  ('social-pressure', '人情账单', 0, 35, 0, 'common', 3, '["social","gift","family"]'::jsonb, '["face","refund"]'::jsonb, 40),
  ('hospital-corridor', '医院走廊', 0, 35, 8000, 'rare', 4, '["medical","pet","deposit"]'::jsonb, '["medical","compensation"]'::jsonb, 50),
  ('admin-window', '行政窗口', 0, 35, 2000, 'common', 2, '["legal","admin","fee"]'::jsonb, '["late-fee","refund"]'::jsonb, 60),
  ('home-renovation', '装修现场', 12000, 35, 40000, 'rare', 4, '["renovation","home","deposit"]'::jsonb, '["rework","damage"]'::jsonb, 70),
  ('pet-er', '宠物急诊夜', 0, 35, 10000, 'rare', 4, '["pet","medical"]'::jsonb, '["medical","compensation"]'::jsonb, 80),
  ('wedding-season', '婚礼季', 0, 35, 20000, 'rare', 4, '["wedding","social","gift"]'::jsonb, '["face","fee"]'::jsonb, 90),
  ('auction-night', '拍卖预展', 12000, 35, 120000, 'wild', 5, '["auction","luxury","mistap"]'::jsonb, '["mistap","fee"]'::jsonb, 100),
  ('masked-party', '面具舞会', 18000, 35, 80000, 'rare', 4, '["luxury","deposit","service-fee"]'::jsonb, '["compensation","fee"]'::jsonb, 110),
  ('travel-chaos', '旅行事故', 8000, 35, 30000, 'rare', 4, '["travel","hotel","flight"]'::jsonb, '["delay","refund","fee"]'::jsonb, 120),
  ('car-owner-day', '车主的一天', 0, 35, 6000, 'common', 3, '["car","traffic","repair"]'::jsonb, '["fine","damage"]'::jsonb, 130),
  ('lucky-counter', '好运柜台', 0, 35, 0, 'rare', 2, '["income","refund","compensation"]'::jsonb, '["income","refund"]'::jsonb, 140),
  ('final-glow', '回光返照', 0, 35, 0, 'wild', 5, '["luxury","medical","ending"]'::jsonb, '["ending","fee"]'::jsonb, 150)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  entry_cost = EXCLUDED.entry_cost,
  duration_sec = EXCLUDED.duration_sec,
  min_balance = EXCLUDED.min_balance,
  rarity = EXCLUDED.rarity,
  risk_level = EXCLUDED.risk_level,
  item_tags = EXCLUDED.item_tags,
  event_tags = EXCLUDED.event_tags,
  sort_order = EXCLUDED.sort_order,
  active = true;

WITH rows (
  id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, tags, flavor, sort_order
) AS (
  VALUES
  ('change-paper-bag','购物袋找零补差','找零阶段',NULL,1,'coin',NULL,true,28,0,'["coin","daily","fee","change"]'::jsonb,'最后一元也会被系统认真收走。',1),
  ('change-bus-transfer','公交换乘补票','找零阶段',NULL,5,'coin',NULL,true,26,0,'["coin","traffic","change"]'::jsonb,'差几块钱的时候，公交卡最懂。',2),
  ('change-powerbank-minute','充电宝超时分钟费','找零阶段',NULL,10,'coin',NULL,true,25,0,'["coin","platform","fee","change"]'::jsonb,'超时一分钟，也算一笔账。',3),
  ('change-parking-start','停车起步补费','找零阶段',NULL,20,'coin',NULL,true,24,0,'["coin","traffic","car","change"]'::jsonb,'余额越少，越需要这种小口子。',4),
  ('change-counter-fee','柜台小额手续费','找零阶段',NULL,50,'coin',NULL,true,23,0,'["coin","admin","fee","change"]'::jsonb,'大钱花完以后，小钱负责收尾。',5),
  ('daily-breakfast-stack','便利店早餐十连','日常小额',NULL,180,'daily',NULL,true,18,0,'["daily","food","batch"]'::jsonb,'小钱最容易被忽略，但连刷会有声音。',10),
  ('soy-milk-spill','豆浆洒键盘','数码意外','repair-week',299,'daily',NULL,false,12,0,'["daily","repair","digital"]'::jsonb,'早餐没喝完，键盘先喝了。',20),
  ('office-coffee-round','办公室咖啡轮值','社交压力',NULL,420,'daily',NULL,true,16,0,'["daily","social","drink"]'::jsonb,'大家说下次请回来，但系统只认本次扣款。',30),
  ('milk-tea-office','奶茶全员请客','社交压力',NULL,486,'daily',NULL,true,18,0,'["social","drink","batch"]'::jsonb,'小额高频，堆起来会很疼。',40),
  ('parking-overtime','停车超时补费','交通',NULL,68,'coin',NULL,true,20,0,'["traffic","car","fee"]'::jsonb,'就是多停了十几分钟。',50),
  ('bike-deposit-missing','共享车押金找回失败','平台规则',NULL,199,'daily',NULL,false,12,0,'["platform","deposit"]'::jsonb,'按钮很多，退款入口很深。',60),
  ('late-night-snacks','深夜零食补给','日常小额','late-night-cart',238,'daily',NULL,true,16,0,'["snack","impulse"]'::jsonb,'嘴上说只买水，购物车不这样想。',70),
  ('lunch-upgrade','工作餐升级套餐','日常小额',NULL,88,'coin',NULL,true,20,0,'["daily","food"]'::jsonb,'多一个小菜，多一笔流水。',80),
  ('weekly-groceries','超市一周补货','日用品',NULL,760,'premium',NULL,true,14,0,'["daily","home","batch"]'::jsonb,'看起来全是刚需。',90),
  ('express-same-day','同城急送跑腿','平台规则',NULL,126,'small',NULL,true,18,0,'["platform","fee","rush"]'::jsonb,'急的时候，服务费也会着急。',100),
  ('ride-surge','跨城打车溢价','交通',NULL,1280,'premium',NULL,true,14,0,'["traffic","surge"]'::jsonb,'临时赶路，价格也临时起飞。',110),
  ('airport-taxi-night','机场夜间接驳','交通','travel-chaos',688,'premium',NULL,true,12,0,'["traffic","travel"]'::jsonb,'飞机落地，账单起飞。',120),
  ('speeding-ticket','区间测速罚单','法律行政','car-owner-day',200,'daily',NULL,false,12,0,'["car","fine","legal"]'::jsonb,'提醒很温柔，扣款很明确。',130),
  ('wrong-lane-fine','走错车道罚单','法律行政','car-owner-day',300,'daily',NULL,false,11,0,'["car","fine","traffic"]'::jsonb,'导航没背锅，钱包背了。',140),
  ('tire-burst-night','夜路爆胎救援','灾难维修','car-owner-day',1680,'premium',NULL,false,11,1000,'["car","repair","damage"]'::jsonb,'轮胎先投降。',150),
  ('car-insurance-gap','车险免赔差额','高端隐形成本','car-owner-day',4200,'large',NULL,false,7,3000,'["car","insurance","fee"]'::jsonb,'保险有用，但不是全包。',160),
  ('phone-replace','手机碎屏换新','数码意外','repair-week',9999,'large',NULL,false,9,5000,'["repair","digital"]'::jsonb,'维修报价让人直接换新。',170),
  ('tablet-water-damage','平板进水检测','数码意外','repair-week',2680,'premium',NULL,false,10,1000,'["repair","digital","damage"]'::jsonb,'检测费不是修好费。',180),
  ('camera-lens-crack','相机镜头裂纹','灾难维修','repair-week',12800,'large',NULL,false,7,8000,'["repair","digital","travel"]'::jsonb,'照片没糊，账单很清楚。',190),
  ('laptop-board-repair','电脑主板维修','数码意外','repair-week',5800,'large',NULL,false,8,3000,'["repair","digital"]'::jsonb,'进度条没动，报价先动。',200),
  ('aircon-leak','空调漏水返修','灾难维修','repair-week',2300,'premium',NULL,false,11,1000,'["repair","home"]'::jsonb,'墙面和心情都湿了。',210),
  ('washer-motor','洗衣机电机更换','灾难维修','repair-week',1890,'premium',NULL,false,10,1000,'["repair","home"]'::jsonb,'衣服没洗完，账单洗出来了。',220),
  ('fridge-emergency','冰箱急修上门','灾难维修','repair-week',1480,'premium',NULL,false,10,1000,'["repair","home","rush"]'::jsonb,'食材在融化，服务费在升温。',230),
  ('wedding-red-envelope','临时份子钱','社交压力','wedding-season',1200,'premium',NULL,true,12,0,'["wedding","social","gift"]'::jsonb,'关系还在，余额少了。',240),
  ('bestman-suit','伴郎礼服押金','社交压力','wedding-season',1800,'premium',NULL,false,9,1000,'["wedding","deposit","social"]'::jsonb,'穿一天，押一笔。',250),
  ('family-banquet','家庭临时请客','社交压力','social-pressure',2688,'premium',NULL,true,10,1000,'["family","social","food"]'::jsonb,'大家都说随便点。',260),
  ('gift-upgrade','伴手礼面子升级','社交压力','social-pressure',688,'premium',NULL,true,13,0,'["gift","social","face"]'::jsonb,'礼轻情意重，系统只看价格。',270),
  ('charity-table','慈善晚宴座位','社交压力','masked-party',6800,'large',NULL,false,5,5000,'["social","gift","luxury"]'::jsonb,'善意很真，席位不便宜。',280),
  ('pet-emergency','宠物急诊押金','宠物','pet-er',16800,'large',NULL,false,8,10000,'["pet","medical","deposit"]'::jsonb,'不做医学建议，只做账单提醒。',290),
  ('pet-ct-scan','宠物影像检查','宠物','pet-er',3600,'large',NULL,false,9,2000,'["pet","medical"]'::jsonb,'小动物安静了，机器开始响。',300),
  ('pet-imported-meds','宠物进口药','宠物','pet-er',2680,'premium',NULL,true,10,1000,'["pet","medical"]'::jsonb,'药瓶很小，价格不小。',310),
  ('pet-surgery-reserve','宠物手术预交','宠物','pet-er',42000,'heavy',1,false,4,30000,'["pet","medical","deposit"]'::jsonb,'先交预付款，再等通知。',320),
  ('dental-root','牙疼根管治疗','健康','hospital-corridor',5600,'large',NULL,false,8,3000,'["medical","health"]'::jsonb,'牙不疼了，钱包开始疼。',330),
  ('allergy-er','过敏急诊检查','健康','hospital-corridor',2100,'premium',NULL,false,10,1000,'["medical","health","rush"]'::jsonb,'不写医学建议，只记录账单。',340),
  ('ambulance-fee','救护车费用','健康','hospital-corridor',980,'premium',NULL,false,8,0,'["medical","transport"]'::jsonb,'车开得很稳，金额也很稳。',350),
  ('hospital-deposit','住院押金','健康','hospital-corridor',30000,'heavy',1,false,5,20000,'["medical","deposit"]'::jsonb,'押金不是总费用，只是开始。',360),
  ('visa-expedite','签证加急费','法律行政','admin-window',1280,'premium',NULL,false,10,0,'["admin","travel","rush"]'::jsonb,'材料没变，费用加急。',370),
  ('id-card-reissue','证件补办加急','法律行政','admin-window',260,'daily',NULL,false,14,0,'["admin","legal","rush"]'::jsonb,'窗口说下一个。',380),
  ('contract-penalty','合同违约金','法律行政','admin-window',8800,'large',NULL,false,6,5000,'["legal","fee"]'::jsonb,'条款很小，数字很大。',390),
  ('late-fee-stack','滞纳金叠加','平台规则','admin-window',618,'premium',NULL,true,11,0,'["late-fee","platform","fee"]'::jsonb,'不是忘了，是系统记得。',400),
  ('auto-renew-year','误买年卡续费','平台规则','late-night-cart',298,'daily',NULL,true,16,0,'["platform","subscription"]'::jsonb,'取消按钮总在角落。',410),
  ('cloud-storage-renew','云空间自动续费','平台规则','late-night-cart',198,'daily',NULL,true,15,0,'["platform","subscription","digital"]'::jsonb,'照片很多，空间不免费。',420),
  ('coupon-expired','优惠券过期反买','平台规则','late-night-cart',460,'daily',NULL,true,13,0,'["coupon","platform"]'::jsonb,'为了不亏，买得更多。',430),
  ('wrong-hotel-nonref','订错不可退酒店','误操作','travel-chaos',6880,'large',NULL,false,7,5000,'["travel","hotel","mistap"]'::jsonb,'日期错了，退款没了。',440),
  ('flight-change','机票改签费','旅行','travel-chaos',2460,'premium',NULL,false,9,1000,'["travel","flight","fee"]'::jsonb,'计划变了，票价也变了。',450),
  ('lost-luggage-kit','行李延误临时采购','旅行','travel-chaos',1180,'premium',NULL,true,11,0,'["travel","delay"]'::jsonb,'东西没到，人先买。',460),
  ('hotel-minibar','酒店迷你吧误拿','旅行','travel-chaos',388,'daily',NULL,true,13,0,'["travel","hotel","mistap"]'::jsonb,'冰箱门很轻，账单很重。',470),
  ('theme-park-fastpass','乐园快速通道','娱乐','travel-chaos',1260,'premium',NULL,true,9,0,'["fun","travel","rush"]'::jsonb,'排队省下了，钱没省下。',480),
  ('concert-chain','演唱会前排连锁','娱乐',NULL,8860,'large',NULL,true,8,8000,'["fun","chain"]'::jsonb,'票、酒店、车费一起入场。',490),
  ('blindbox-hidden','盲盒隐藏款加购','中奖捡漏','late-night-cart',159,'daily',NULL,true,15,0,'["blindbox","impulse"]'::jsonb,'差一个就齐，永远差一个。',500),
  ('resale-flip-fail','二手转卖亏差','中奖捡漏','late-night-cart',720,'premium',NULL,false,8,0,'["resale","mistap"]'::jsonb,'以为能回本，结果补差价。',510),
  ('antique-appraisal','古玩鉴定服务费','中奖捡漏','auction-night',2800,'premium',NULL,false,7,2000,'["auction","resale","fee"]'::jsonb,'故事很长，证书很贵。',520),
  ('renovation-rework','装修返工增项','大件现实','home-renovation',68000,'heavy',NULL,false,5,40000,'["renovation","rework","home"]'::jsonb,'敲开墙，也敲开预算。',530),
  ('custom-cabinet-add','定制柜增项','大件现实','home-renovation',22000,'large',NULL,false,6,15000,'["renovation","home"]'::jsonb,'多一块板，多一行报价。',540),
  ('floor-relevel','地面找平补差','大件现实','home-renovation',13800,'large',NULL,false,7,9000,'["renovation","home","rework"]'::jsonb,'水平线很平，账单不平。',550),
  ('moving-weekend','周末搬家加价','大件现实','home-renovation',3600,'large',NULL,true,9,2000,'["home","service-fee","rush"]'::jsonb,'大家都选周末。',560),
  ('new-sofa-deposit','沙发定金','大件现实','home-renovation',5200,'large',NULL,false,8,3000,'["home","deposit"]'::jsonb,'坐上去舒服，付款时清醒。',570),
  ('appliance-suite','家电套装尾款','大件现实','home-renovation',36800,'heavy',NULL,false,5,25000,'["home","appliance"]'::jsonb,'套装优惠不代表便宜。',580),
  ('parking-space-deposit','车位意向金','大件现实','car-owner-day',50000,'heavy',1,false,4,35000,'["car","home","deposit"]'::jsonb,'车还没停，钱先停了。',590),
  ('auction-mistap','拍卖误举牌','高端误操作','auction-night',188000,'shock',1,false,3,120000,'["auction","mistap","luxury"]'::jsonb,'手举起来，钱落下去。',600),
  ('auction-buyer-premium','拍卖佣金补差','高端隐形成本','auction-night',32000,'heavy',1,false,4,20000,'["auction","fee","luxury"]'::jsonb,'锤子落下之后还有比例。',610),
  ('private-dinner-seat','私人晚宴席位','富人体验','masked-party',25800,'heavy',1,false,4,18000,'["luxury","social","food"]'::jsonb,'不是吃饭，是入场。',620),
  ('yacht-cleaning','游艇清洁赔补','富人体验','masked-party',96000,'heavy',1,false,4,80000,'["luxury","service-fee"]'::jsonb,'租赁没买贵，服务费贵。',630),
  ('castle-deposit','古堡酒店定金','富人体验','masked-party',128000,'shock',1,false,3,100000,'["luxury","deposit","hotel"]'::jsonb,'住不起没关系，定金先行。',640),
  ('polar-cruise-deposit','极地邮轮订金','富人体验','travel-chaos',86000,'heavy',1,false,4,70000,'["luxury","travel","deposit"]'::jsonb,'离南极还远，离扣款很近。',650),
  ('europe-roadtrip-hold','欧洲自驾押金','富人体验','travel-chaos',42000,'heavy',1,false,4,30000,'["travel","luxury","deposit"]'::jsonb,'押金会回来吗，先另说。',660),
  ('luxury-cleaning-fee','高级清洁费','高端隐形成本','masked-party',6800,'large',NULL,false,7,5000,'["luxury","service-fee"]'::jsonb,'清洁的是场地，不是余额。',670),
  ('warehouse-storage','仓储保管费','高端隐形成本','home-renovation',3200,'large',NULL,true,8,2000,'["storage","fee","home"]'::jsonb,'东西没到家，钱先住仓库。',680),
  ('rush-customs','清关加急费','高端隐形成本','travel-chaos',4600,'large',NULL,false,7,3000,'["travel","admin","rush"]'::jsonb,'海关不急，你急。',690),
  ('wrong-set-meal','点错豪华套餐','误操作','late-night-cart',1980,'premium',NULL,false,9,1000,'["mistap","food","impulse"]'::jsonb,'图片很诱人，确认键很危险。',700),
  ('duplicate-order','重复下单补救失败','误操作','late-night-cart',760,'premium',NULL,true,12,0,'["mistap","platform"]'::jsonb,'取消中，已出库。',710),
  ('accidental-private-room','手滑包场','误操作','masked-party',45000,'heavy',1,false,3,30000,'["mistap","luxury","social"]'::jsonb,'你以为选了座位，其实选了全场。',720),
  ('tax-refund','退税突然到账','反向进账','lucky-counter',22000,'income',NULL,false,7,0,'["income","refund"]'::jsonb,'不是奖励，是阻碍清空。',730),
  ('hotel-compensation','酒店超售赔付','赔付','lucky-counter',8000,'income',NULL,false,8,0,'["income","compensation","hotel"]'::jsonb,'钱回来了，通关远了。',740),
  ('friend-payback','朋友突然还钱','反向进账','lucky-counter',3500,'income',NULL,false,9,0,'["income","social"]'::jsonb,'多年旧账，在最不该来的时候来了。',750),
  ('deposit-return','押金原路退回','反向进账','lucky-counter',12000,'income',NULL,false,8,0,'["income","deposit","refund"]'::jsonb,'原路返回，清空倒退。',760),
  ('shipping-insurance','运费险到账','反向进账','lucky-counter',88,'income',NULL,true,14,0,'["income","refund","platform"]'::jsonb,'小额返钱也会打断节奏。',770),
  ('annual-party-prize','年会中奖','中奖捡漏','lucky-counter',30000,'income',NULL,false,5,0,'["income","luck"]'::jsonb,'中奖不是胜利，是余额反弹。',780),
  ('lottery-small-win','小奖到账','中奖捡漏','lucky-counter',5000,'income',NULL,false,7,0,'["income","luck"]'::jsonb,'概率很低，但来了就很烦。',790),
  ('insurance-claim','保险理赔到账','赔付','lucky-counter',18000,'income',NULL,false,6,0,'["income","insurance","compensation"]'::jsonb,'赔付很合理，目标很受伤。',800)
)
INSERT INTO content_items (
  id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, modes, tags, flavor, sort_order
)
SELECT id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, '["chaos-life"]'::jsonb, tags, flavor, sort_order
FROM rows
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  category = EXCLUDED.category,
  scene_id = EXCLUDED.scene_id,
  price = EXCLUDED.price,
  tier = EXCLUDED.tier,
  max_buy = EXCLUDED.max_buy,
  batchable = EXCLUDED.batchable,
  weight = EXCLUDED.weight,
  min_balance = EXCLUDED.min_balance,
  modes = EXCLUDED.modes,
  tags = EXCLUDED.tags,
  flavor = EXCLUDED.flavor,
  sort_order = EXCLUDED.sort_order,
  active = true;

INSERT INTO content_events (
  id, title, description, delta, probability, cooldown_sec, tags, settlement_tag, sort_order
) VALUES
  ('refund-sting','反向进账','退款到账，清空进度被拖回。',8000,0.12,8,'["income","refund"]'::jsonb,'最烦人返钱',10),
  ('rush-fee','加急服务费','高压状态下追加服务费。',-12000,0.16,6,'["fee","rush"]'::jsonb,'最荒诞扣款',20),
  ('coupon-bounce','优惠券返现','平台突然返了一张现金券。',3000,0.10,10,'["income","coupon"]'::jsonb,'最烦人返钱',30),
  ('damage-followup','损坏追缴','商家追加损坏处理费。',-6800,0.11,10,'["damage","fee"]'::jsonb,'最荒诞扣款',40),
  ('delay-compensation','延误赔付','行程延误获得一笔赔付。',6000,0.08,12,'["income","delay","compensation"]'::jsonb,'最烦人返钱',50),
  ('face-upgrade','面子升级','临场被动升级规格。',-5200,0.10,8,'["social","face"]'::jsonb,'最荒诞扣款',60),
  ('medical-recheck','复查提醒','又补了一项检查费用。',-2600,0.10,9,'["medical","fee"]'::jsonb,'最荒诞扣款',70),
  ('deposit-return-event','押金退回','押金突然原路退回。',12000,0.07,12,'["income","deposit"]'::jsonb,'最烦人返钱',80),
  ('admin-late-fee','窗口滞纳','错过时间产生滞纳金。',-1800,0.10,8,'["admin","late-fee"]'::jsonb,'最荒诞扣款',90),
  ('platform-subsidy','平台补贴','系统补贴到账。',2500,0.10,8,'["income","platform"]'::jsonb,'最烦人返钱',100),
  ('cleaning-claim','清洁索赔','场地追加清洁索赔。',-9800,0.08,10,'["service-fee","luxury"]'::jsonb,'最荒诞扣款',110),
  ('resale-success','二手转卖成功','有人接盘，现金回流。',9000,0.06,14,'["income","resale"]'::jsonb,'最烦人返钱',120)
ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  description = EXCLUDED.description,
  delta = EXCLUDED.delta,
  probability = EXCLUDED.probability,
  cooldown_sec = EXCLUDED.cooldown_sec,
  tags = EXCLUDED.tags,
  settlement_tag = EXCLUDED.settlement_tag,
  sort_order = EXCLUDED.sort_order,
  active = true;

INSERT INTO content_statuses (
  id, name, duration_sec, item_refresh_multiplier, high_price_multiplier, event_multiplier, tags, description, sort_order
) VALUES
  ('rage-buy','生气',12,1.15,1.30,1.10,'["impulse","auction","high-risk","big-spend"]'::jsonb,'高价消费更容易出现。',10),
  ('lucky-backfire','好运',10,1.00,0.80,1.40,'["income","refund","compensation","income-scene"]'::jsonb,'返钱和赔付更容易出现。',20),
  ('compare-mode','攀比',14,1.05,1.25,1.10,'["social","gift","luxury","face"]'::jsonb,'礼物、请客和会籍概率上升。',30),
  ('hoard-anxiety','焦虑囤货',14,1.15,0.90,1.00,'["daily","home","food","child"]'::jsonb,'日用品和食品批量出现。',40),
  ('numb-shopping','购物麻木',16,1.00,1.00,0.95,'["daily","platform","subscription"]'::jsonb,'正常扣钱但结算更密集。',50),
  ('revenge-spend','报复消费',12,1.20,1.45,0.85,'["impulse","luxury","high-risk","late-game"]'::jsonb,'高价刷新上升，返钱变少。',60),
  ('palpitation','心悸',8,0.90,1.00,1.25,'["medical","health","pet","high-risk"]'::jsonb,'医疗和宠物急诊类事件概率上升。',70),
  ('fatigue','疲劳',14,0.85,0.95,1.00,'["late-game","low-balance","rush"]'::jsonb,'刷新速度下降。',80),
  ('unlucky','倒霉',12,1.00,1.05,1.30,'["repair","damage","fine","fee"]'::jsonb,'损坏、罚单、加价更容易出现。',90),
  ('final-glow','回光返照',10,1.25,1.60,1.20,'["low-balance","late-game","shock"]'::jsonb,'终局前高消费商品暴涨。',100)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  duration_sec = EXCLUDED.duration_sec,
  item_refresh_multiplier = EXCLUDED.item_refresh_multiplier,
  high_price_multiplier = EXCLUDED.high_price_multiplier,
  event_multiplier = EXCLUDED.event_multiplier,
  tags = EXCLUDED.tags,
  description = EXCLUDED.description,
  sort_order = EXCLUDED.sort_order,
  active = true;

INSERT INTO audio_tracks (
  id, title, mood, src, license, source_url, sort_order
) VALUES
  ('rush','内置合成收银循环','rush','','custom','',10),
  ('danger','内置合成高压循环','danger','','custom','',20),
  ('settlement','内置合成结算循环','settlement','','custom','',30)
ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  mood = EXCLUDED.mood,
  src = EXCLUDED.src,
  license = EXCLUDED.license,
  source_url = EXCLUDED.source_url,
  sort_order = EXCLUDED.sort_order,
  active = true;

INSERT INTO content_scenes (
  id, name, entry_cost, duration_sec, min_balance, rarity, risk_level, item_tags, event_tags, sort_order
) VALUES
  ('campus-wallet', '校园钱包', 0, 35, 0, 'common', 2, '["daily","education","social"]'::jsonb, '["refund","fee"]'::jsonb, 160),
  ('parenting-week', '带娃周末', 0, 35, 1000, 'common', 3, '["child","education","medical"]'::jsonb, '["fee","compensation"]'::jsonb, 170),
  ('office-promotion', '职场晋升局', 0, 35, 3000, 'rare', 3, '["career","social","gift"]'::jsonb, '["face","refund"]'::jsonb, 180),
  ('midlife-checkup', '中年体检单', 0, 35, 5000, 'rare', 4, '["medical","insurance","family"]'::jsonb, '["medical","compensation"]'::jsonb, 190),
  ('luxury-afterparty', '奢华散场后', 20000, 35, 100000, 'wild', 5, '["luxury","service-fee","travel"]'::jsonb, '["fee","damage","compensation"]'::jsonb, 200)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  entry_cost = EXCLUDED.entry_cost,
  duration_sec = EXCLUDED.duration_sec,
  min_balance = EXCLUDED.min_balance,
  rarity = EXCLUDED.rarity,
  risk_level = EXCLUDED.risk_level,
  item_tags = EXCLUDED.item_tags,
  event_tags = EXCLUDED.event_tags,
  sort_order = EXCLUDED.sort_order,
  active = true;

WITH item_templates (
  template_id, base_name, category, scene_id, base_price, price_step, min_balance, tags, flavor, sort_order
) AS (
  VALUES
  ('food', '临时餐饮账单', '日常小额', 'daily-loop', 96, 42, 0, '["daily","food"]'::jsonb, '嘴上说随便吃点，账单自己长大。', 1000),
  ('drink', '饮料咖啡补给', '日常小额', 'daily-loop', 68, 38, 0, '["daily","drink","social"]'::jsonb, '饮品很快喝完，流水留下来了。', 1100),
  ('traffic', '出行临时加价', '交通', 'car-owner-day', 180, 210, 0, '["traffic","rush"]'::jsonb, '路程不远，费用会绕路。', 1200),
  ('platform', '平台规则扣款', '平台规则', 'late-night-cart', 128, 96, 0, '["platform","subscription","fee"]'::jsonb, '规则写得很细，玩家读得很快。', 1300),
  ('home', '家务维修补单', '灾难维修', 'repair-week', 520, 360, 0, '["home","repair","damage"]'::jsonb, '不是大事故，但每次都要钱。', 1400),
  ('digital', '数码设备意外', '数码意外', 'repair-week', 880, 520, 0, '["digital","repair","damage"]'::jsonb, '屏幕亮着，钱包暗了。', 1500),
  ('social', '社交人情加码', '社交压力', 'social-pressure', 360, 330, 0, '["social","gift","face"]'::jsonb, '面子没有标价，但这里有。', 1600),
  ('child', '带娃临时开销', '亲子教育', 'parenting-week', 220, 420, 0, '["child","education","family"]'::jsonb, '小朋友开心了，大人沉默了。', 1700),
  ('campus', '学习考试费用', '教育考试', 'campus-wallet', 160, 260, 0, '["education","admin"]'::jsonb, '学习使人进步，也使余额移动。', 1800),
  ('career', '职场形象成本', '职场晋升', 'office-promotion', 680, 620, 1000, '["career","social","face"]'::jsonb, '升职还没到，成本先到。', 1900),
  ('medical', '健康检查加项', '健康', 'midlife-checkup', 900, 880, 1000, '["medical","health","insurance"]'::jsonb, '不写医学建议，只记录费用变化。', 2000),
  ('pet', '宠物照护补项', '宠物', 'pet-er', 760, 740, 1000, '["pet","medical"]'::jsonb, '小动物不会讲价。', 2100),
  ('admin', '行政窗口费用', '法律行政', 'admin-window', 240, 380, 0, '["admin","legal","fee"]'::jsonb, '窗口不大，费用不少。', 2200),
  ('travel', '旅行意外消费', '旅行', 'travel-chaos', 980, 1260, 2000, '["travel","hotel","flight"]'::jsonb, '行程变化永远比计划贵。', 2300),
  ('renovation', '装修现场增项', '大件现实', 'home-renovation', 3600, 4200, 8000, '["renovation","home","rework"]'::jsonb, '每一项都说是最后一次。', 2400),
  ('wedding', '婚礼季账单', '社交压力', 'wedding-season', 1200, 1800, 2000, '["wedding","social","gift"]'::jsonb, '祝福很真，金额也真。', 2500),
  ('luxury', '高端体验尾款', '富人体验', 'luxury-afterparty', 8800, 9600, 20000, '["luxury","service-fee"]'::jsonb, '入场费只是门口。', 2600),
  ('auction', '拍卖周边成本', '高端误操作', 'auction-night', 6800, 8800, 20000, '["auction","luxury","mistap"]'::jsonb, '锤子没落下时也会收费。', 2700),
  ('car', '车主隐形成本', '交通', 'car-owner-day', 460, 760, 0, '["car","traffic","repair"]'::jsonb, '车在路上，钱在路上。', 2800),
  ('income', '反向到账', '反向进账', 'lucky-counter', 300, 820, 0, '["income","refund","compensation"]'::jsonb, '这笔钱回来得很不是时候。', 2900)
),
variants (variant_index, variant_name, price_multiplier, weight_delta) AS (
  VALUES
  (1, '基础款', 1.00, 9),
  (2, '加急款', 1.25, 8),
  (3, '升级款', 1.60, 7),
  (4, '押金款', 2.10, 6),
  (5, '套餐款', 2.80, 6),
  (6, '周末款', 3.60, 5),
  (7, '误操作款', 4.80, 4),
  (8, '保价失败款', 6.20, 4),
  (9, '高峰款', 8.20, 3),
  (10, '隐藏成本款', 10.80, 3),
  (11, '离谱款', 14.50, 2)
),
expanded AS (
  SELECT
    'auto-card-' || template_id || '-' || variant_index AS id,
    base_name || '·' || variant_name AS name,
    category,
    scene_id,
    GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier))::BIGINT AS price,
    CASE
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 150 THEN 'coin'
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 350 THEN 'small'
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 800 THEN 'daily'
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 3000 THEN 'premium'
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 20000 THEN 'large'
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 90000 THEN 'heavy'
      ELSE 'shock'
    END AS tier,
    CASE
      WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) >= 60000 THEN 1::BIGINT
      ELSE NULL::BIGINT
    END AS max_buy,
    template_id <> 'income' AND GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) <= 5000 AS batchable,
    GREATEST(1, weight_delta + CASE WHEN template_id = 'income' THEN 2 ELSE 0 END)::BIGINT AS weight,
    GREATEST(min_balance, (GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) / 4)::BIGINT) AS min_balance,
    CASE WHEN template_id = 'income' THEN 'income' ELSE
      CASE
        WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 150 THEN 'coin'
        WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 350 THEN 'small'
        WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 800 THEN 'daily'
        WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 3000 THEN 'premium'
        WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 20000 THEN 'large'
        WHEN GREATEST(1, ROUND((base_price + price_step * (variant_index - 1)) * price_multiplier)) < 90000 THEN 'heavy'
        ELSE 'shock'
      END
    END AS final_tier,
    '["chaos-life"]'::jsonb AS modes,
    tags || jsonb_build_array(variant_name) AS tags,
    flavor,
    sort_order + variant_index AS sort_order
  FROM item_templates
  CROSS JOIN variants
)
INSERT INTO content_items (
  id, name, category, scene_id, price, tier, max_buy, batchable, weight, min_balance, modes, tags, flavor, sort_order
)
SELECT
  id,
  name,
  category,
  scene_id,
  LEAST(price, 320000)::BIGINT,
  CASE
    WHEN id LIKE 'auto-card-income-%' THEN 'income'
    WHEN LEAST(price, 320000) < 150 THEN 'coin'
    WHEN LEAST(price, 320000) < 350 THEN 'small'
    WHEN LEAST(price, 320000) < 800 THEN 'daily'
    WHEN LEAST(price, 320000) < 3000 THEN 'premium'
    WHEN LEAST(price, 320000) < 20000 THEN 'large'
    WHEN LEAST(price, 320000) < 90000 THEN 'heavy'
    ELSE 'shock'
  END,
  CASE
    WHEN id LIKE 'auto-card-income-%' THEN NULL::BIGINT
    WHEN LEAST(price, 320000) >= 60000 THEN 1::BIGINT
    ELSE max_buy
  END,
  batchable,
  weight,
  LEAST(min_balance, GREATEST(0, (LEAST(price, 320000) / 3)::BIGINT)),
  modes,
  tags,
  flavor,
  sort_order
FROM expanded
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  category = EXCLUDED.category,
  scene_id = EXCLUDED.scene_id,
  price = EXCLUDED.price,
  tier = EXCLUDED.tier,
  max_buy = EXCLUDED.max_buy,
  batchable = EXCLUDED.batchable,
  weight = EXCLUDED.weight,
  min_balance = EXCLUDED.min_balance,
  modes = EXCLUDED.modes,
  tags = EXCLUDED.tags,
  flavor = EXCLUDED.flavor,
  sort_order = EXCLUDED.sort_order,
  active = true;

WITH event_templates (
  template_id, title, description, base_delta, probability, cooldown_sec, tags, settlement_tag, sort_order
) AS (
  VALUES
  ('refund', '退款回流', '平台退款突然到账，清空进度被拉回。', 1800, 0.10, 8, '["income","refund","platform"]'::jsonb, '最烦人返钱', 1000),
  ('cashback', '会员返现', '系统返现到账，余额反而更厚。', 900, 0.10, 8, '["income","platform"]'::jsonb, '最烦人返钱', 1010),
  ('deposit', '押金退回', '押金原路退回，之前的努力被抵消。', 4200, 0.08, 10, '["income","deposit"]'::jsonb, '最烦人返钱', 1020),
  ('compensation', '赔付到账', '一次小事故换来反向进账。', 3200, 0.08, 10, '["income","compensation"]'::jsonb, '最烦人返钱', 1030),
  ('lottery', '抽奖中奖', '低概率小奖到账，目标被拖远。', 5200, 0.06, 12, '["income","luck"]'::jsonb, '最烦人返钱', 1040),
  ('payback', '朋友还钱', '旧账突然还清，余额反弹。', 2600, 0.07, 10, '["income","social"]'::jsonb, '最烦人返钱', 1050),
  ('delay', '延误补偿', '行程延误获得补偿。', 3600, 0.07, 12, '["income","travel","delay"]'::jsonb, '最烦人返钱', 1060),
  ('insurance', '理赔到账', '保险理赔到账，但通关变远。', 7800, 0.06, 14, '["income","insurance","medical"]'::jsonb, '最烦人返钱', 1070),
  ('rush', '加急追加', '临时加急产生额外服务费。', -2200, 0.12, 8, '["fee","rush"]'::jsonb, '最荒诞扣款', 1080),
  ('damage', '损坏追缴', '后续损坏费用被追加。', -4600, 0.10, 10, '["damage","repair"]'::jsonb, '最荒诞扣款', 1090),
  ('cleaning', '清洁索赔', '场地或设备追加清洁费用。', -6800, 0.09, 10, '["service-fee","luxury"]'::jsonb, '最荒诞扣款', 1100),
  ('admin', '行政补费', '窗口规则触发补费。', -1200, 0.11, 8, '["admin","legal","fee"]'::jsonb, '最荒诞扣款', 1110),
  ('late', '滞纳叠加', '错过时限产生滞纳金。', -900, 0.11, 8, '["late-fee","platform"]'::jsonb, '最荒诞扣款', 1120),
  ('face', '面子升级', '现场氛围让规格被动升级。', -3800, 0.10, 9, '["social","face","gift"]'::jsonb, '最荒诞扣款', 1130),
  ('medical', '检查补项', '健康相关项目又补了一项费用。', -3200, 0.10, 9, '["medical","health"]'::jsonb, '最荒诞扣款', 1140),
  ('pet-followup', '宠物复诊补项', '宠物急诊之后又补了一项照护费用。', -3600, 0.09, 10, '["pet","medical","fee"]'::jsonb, '最荒诞扣款', 1145),
  ('pet-insurance', '宠物保险赔付', '宠物保险赔付到账，清空进度被拖远。', 4200, 0.06, 14, '["pet","income","insurance","compensation"]'::jsonb, '最烦人返钱', 1146),
  ('travel-fee', '行程补差', '行程变化带来补差价。', -5200, 0.09, 10, '["travel","fee"]'::jsonb, '最荒诞扣款', 1150),
  ('auction-fee', '高端佣金', '高端场景追加比例费用。', -12000, 0.07, 12, '["auction","luxury","fee"]'::jsonb, '最荒诞扣款', 1160)
),
event_variants (variant_index, suffix, multiplier, probability_delta) AS (
  VALUES
  (1, '轻微', 1.00, 0.00),
  (2, '升级', 1.80, 0.01),
  (3, '连锁', 2.80, 0.015),
  (4, '离谱', 4.20, 0.02)
)
INSERT INTO content_events (
  id, title, description, delta, probability, cooldown_sec, tags, modes, settlement_tag, sort_order
)
SELECT
  'auto-event-' || template_id || '-' || variant_index,
  title || '·' || suffix,
  description,
  ROUND(base_delta * multiplier)::BIGINT,
  LEAST(0.45, probability + probability_delta),
  cooldown_sec + variant_index,
  tags || jsonb_build_array(suffix),
  '["chaos-life"]'::jsonb,
  settlement_tag,
  sort_order + variant_index
FROM event_templates
CROSS JOIN event_variants
ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  description = EXCLUDED.description,
  delta = EXCLUDED.delta,
  probability = EXCLUDED.probability,
  cooldown_sec = EXCLUDED.cooldown_sec,
  tags = EXCLUDED.tags,
  modes = EXCLUDED.modes,
  settlement_tag = EXCLUDED.settlement_tag,
  sort_order = EXCLUDED.sort_order,
  active = true;

INSERT INTO content_statuses (
  id, name, duration_sec, item_refresh_multiplier, high_price_multiplier, event_multiplier, tags, description, sort_order
) VALUES
  ('low-mood', '低落', 14, 0.80, 0.90, 1.05, '["low-mood","daily","income","refund","low-balance"]'::jsonb, '刷新变慢，限时高压场景会临时退回普通货架。', 110),
  ('hype-spree', '上头', 10, 1.25, 1.35, 1.20, '["impulse","mistap","platform","high-risk","rush"]'::jsonb, '批量和误操作风险上升，货架节奏变得更急。', 120)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  duration_sec = EXCLUDED.duration_sec,
  item_refresh_multiplier = EXCLUDED.item_refresh_multiplier,
  high_price_multiplier = EXCLUDED.high_price_multiplier,
  event_multiplier = EXCLUDED.event_multiplier,
  tags = EXCLUDED.tags,
  description = EXCLUDED.description,
  sort_order = EXCLUDED.sort_order,
  active = true;

INSERT INTO content_endings (
  id, title, description, probability, min_elapsed_ms, max_balance, min_risk_level, balance_effect, tags, settlement_tag, sort_order
) VALUES
  (
    'heart-alarm-blackout',
    '心脏报警停表',
    '系统提示你先离开收银台处理身体报警。本局提前停表，只记录账单，不提供医学建议。',
    0.0005,
    240000,
    NULL,
    4,
    'none',
    '["medical","health","ending"]'::jsonb,
    '心脏报警终局',
    10
  ),
  (
    'emergency-observation',
    '急诊留观托管',
    '急诊留观把操作权托管给系统，剩余余额原样进入特殊战报。',
    0.0006,
    210000,
    900000,
    3,
    'none',
    '["medical","deposit","ending"]'::jsonb,
    '急诊留观终局',
    20
  ),
  (
    'accident-heavy-settlement',
    '事故重伤结算',
    '突发事故让本局提前结算。这里记录的是虚构账单压力，不评价真实事故。',
    0.0004,
    300000,
    NULL,
    4,
    'none',
    '["travel","car","damage","ending"]'::jsonb,
    '事故重伤终局',
    30
  ),
  (
    'coma-hospital-custody',
    '昏迷住院托管',
    '系统进入住院托管模式，购物回合提前结束，后续只生成战报。',
    0.0003,
    360000,
    650000,
    4,
    'none',
    '["medical","insurance","ending"]'::jsonb,
    '住院托管终局',
    40
  ),
  (
    'bankruptcy-zero',
    '破产式清零',
    '一连串费用把剩余额度直接归零。主线目标达成，但结算会标记为特殊终局。',
    0.0007,
    300000,
    280000,
    3,
    'zero',
    '["fee","legal","late-fee","ending"]'::jsonb,
    '破产式清零',
    50
  ),
  (
    'life-screen-off',
    '人生突然黑屏',
    '不是医学建议，也不把死亡当笑点；系统只是在荒诞账单里按下停表键。',
    0.00025,
    390000,
    NULL,
    5,
    'none',
    '["ending","luxury","medical"]'::jsonb,
    '突然黑屏终局',
    60
  )
ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  description = EXCLUDED.description,
  probability = EXCLUDED.probability,
  min_elapsed_ms = EXCLUDED.min_elapsed_ms,
  max_balance = EXCLUDED.max_balance,
  min_risk_level = EXCLUDED.min_risk_level,
  balance_effect = EXCLUDED.balance_effect,
  tags = EXCLUDED.tags,
  settlement_tag = EXCLUDED.settlement_tag,
  sort_order = EXCLUDED.sort_order,
  active = true;
