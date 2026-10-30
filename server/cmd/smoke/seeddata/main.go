// 种子数据工具：清空业务数据后灌入贴近真实场景的测试数据。
// 保留：表结构、admin 超管账号、核销规则。
// 用法：go run ./cmd/smoke/seeddata   （可重复执行，每次先清空再灌）
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"canteen/server/internal/auth"
	"github.com/jackc/pgx/v5"
)

var dsn = envDefault("DATABASE_URL",
	"postgresql://postgres.yikizzibtsokolgnnkfi:MyDb2026Secure!789@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require")

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var rng = rand.New(rand.NewSource(20261030))

func pick[T any](arr []T) T   { return arr[rng.Intn(len(arr))] }
func prob(p float64) bool     { return rng.Float64() < p }
func rint(n int) int          { return rng.Intn(n) }

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	/* ---------- 1. 清空业务数据 ---------- */
	for _, stmt := range []string{
		`TRUNCATE verifications, walk_in_records, dish_reviews, meal_signups, suggestions, menu_items RESTART IDENTITY CASCADE`,
		`DELETE FROM users WHERE role <> 'super_admin'`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			fmt.Fprintln(os.Stderr, "wipe:", err)
			os.Exit(1)
		}
	}
	fmt.Println("[1/5] 旧业务数据已清空（admin 账号保留）")

	/* ---------- 2. 用户 ---------- */
	hash, _ := auth.HashPassword("123456")
	batch := &pgx.Batch{}

	type emp struct{ no, name, dept string }
	depts := []string{"生产一部", "生产二部", "质检部", "仓储物流", "行政部", "研发部"}
	surnames := []string{"王", "李", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴", "徐", "孙", "马", "朱", "胡", "郭", "何", "林", "罗", "郑"}
	givens := []string{"伟", "芳", "娜", "敏", "静", "磊", "军", "洋", "勇", "艳", "杰", "涛", "明", "超", "秀兰", "霞", "平", "刚", "桂英", "文轩", "雨欣", "浩然", "梓涵", "子轩", "一诺", "思远", "佳琪", "鑫", "磊磊", "建国"}

	// 食堂管理员 2 名
	adminIDs := []int64{}
	for _, a := range []struct{ no, name string }{{"canteen01", "张管理"}, {"canteen02", "李管理"}} {
		var id int64
		if err := conn.QueryRow(ctx, `insert into users (emp_no,name,department,password_hash,role,status,must_change_password)
			values ($1,$2,'后勤部',$3,'canteen_admin','active',false) returning id`, a.no, a.name, hash).Scan(&id); err != nil {
			fmt.Fprintln(os.Stderr, "admin:", err)
			os.Exit(1)
		}
		adminIDs = append(adminIDs, id)
	}
	// 员工 30 名（含 1 个禁用账号用于账号管理页展示）
	empList := []emp{}
	for i := 0; i < 30; i++ {
		no := fmt.Sprintf("E%03d", i+1)
		name := surnames[i%len(surnames)] + givens[i%len(givens)]
		dept := depts[i%len(depts)]
		empList = append(empList, emp{no, name, dept})
		status := "active"
		if i == 29 {
			status = "disabled" // E030 禁用态，供账号管理页展示
		}
		batch.Queue(`insert into users (emp_no,name,department,password_hash,role,status,must_change_password)
			values ($1,$2,$3,$4,'employee',$5,false)`, no, name, dept, hash, status)
	}
	if err := conn.SendBatch(ctx, batch).Close(); err != nil {
		fmt.Fprintln(os.Stderr, "users:", err)
		os.Exit(1)
	}
	// 取回员工 ID
	rows, _ := conn.Query(ctx, `select id, emp_no from users where role='employee' order by id`)
	empIDs := map[string]int64{}
	for rows.Next() {
		var id int64
		var no string
		_ = rows.Scan(&id, &no)
		empIDs[no] = id
	}
	rows.Close()
	fmt.Printf("[2/5] 用户就绪：管理员 2 名 + 员工 30 名（E030 为禁用态），密码均为 123456\n")

	/* ---------- 3. 菜谱：过去 10 天 + 今天 + 未来 14 天 ---------- */
	breakfastPool := []struct{ n, t string }{
		{"鲜肉包子", "咸鲜"}, {"素三鲜包", "清淡"}, {"油条", "香脆"}, {"豆浆", "原味"},
		{"茶叶蛋", "五香"}, {"小米粥", "清淡"}, {"煎饼果子", "咸鲜"}, {"皮蛋瘦肉粥", "咸鲜"},
	}
	lunchPool := []struct{ n, t string }{
		{"红烧排骨", "咸鲜"}, {"宫保鸡丁", "微辣"}, {"鱼香肉丝", "酸甜"}, {"清炒时蔬", "清淡"},
		{"番茄炒蛋", "酸甜"}, {"红烧鱼块", "咸鲜"}, {"糖醋里脊", "酸甜"}, {"麻婆豆腐", "麻辣"},
		{"可乐鸡翅", "甜咸"}, {"蒜蓉西兰花", "清淡"}, {"土豆炖牛腩", "咸鲜"}, {"青椒肉丝", "微辣"},
	}
	dinnerPool := []struct{ n, t string }{
		{"扬州炒饭", "咸鲜"}, {"西红柿鸡蛋面", "酸甜"}, {"香菇滑鸡煲", "咸鲜"}, {"凉拌黄瓜", "清爽"},
		{"酸菜鱼", "酸辣"}, {"韭菜盒子", "咸鲜"}, {"肉末茄子", "咸鲜"}, {"冬瓜排骨汤", "清淡"},
	}

	days := 25 // -10 ~ +14
	menuCount := 0
	batch = &pgx.Batch{}
	for d := -10; d <= 14; d++ {
		date := time.Now().AddDate(0, 0, d).Format("2006-01-02")
		for mealI, meal := range []string{"breakfast", "lunch", "dinner"} {
			pool := []struct{ n, t string }{}
			switch mealI {
			case 0:
				pool = breakfastPool
			case 1:
				pool = lunchPool
			default:
				pool = dinnerPool
			}
			n := 3 + rint(2)
			chosen := map[string]bool{}
			for i := 0; i < n; i++ {
				dish := pool[rint(len(pool))]
				if chosen[dish.n] {
					continue
				}
				chosen[dish.n] = true
				batch.Queue(`insert into menu_items (menu_date,meal,dish_name,taste_tags) values ($1::date,$2::meal_type,$3,array[$4])`,
					date, meal, dish.n, dish.t)
				menuCount++
			}
		}
	}
	if err := conn.SendBatch(ctx, batch).Close(); err != nil {
		fmt.Fprintln(os.Stderr, "menu:", err)
		os.Exit(1)
	}
	// 每天每餐的菜单按日期存起来（供报名/点评关联）
	dayMenus := map[string]map[string][]string{} // date -> meal -> dishes
	for d := -10; d <= 14; d++ {
		date := time.Now().AddDate(0, 0, d).Format("2006-01-02")
		dayMenus[date] = map[string][]string{
			"breakfast": {"鲜肉包子", "豆浆", "茶叶蛋"},
			"lunch":     {"红烧排骨", "宫保鸡丁", "清炒时蔬", "番茄炒蛋"},
			"dinner":    {"扬州炒饭", "西红柿鸡蛋面", "凉拌黄瓜"},
		}
	}
	fmt.Printf("[3/5] 菜谱就绪：%d 天 × 三餐，共 %d 道菜\n", days, menuCount)

	/* ---------- 4. 报餐 / 核销 / 违约 / 未报名就餐 / 点评 ---------- */
	deadlines := map[string]int{"breakfast": 9, "lunch": 13, "dinner": 19}
	signupCount, verifyCount, noShowCount, cancelCount, walkInCount, reviewCount := 0, 0, 0, 0, 0, 0

	insSignup := func(uid int64, date, meal string, signedAt time.Time) int64 {
		var id int64
		_ = conn.QueryRow(ctx, `insert into meal_signups (user_id,menu_date,meal,status,signed_at)
			values ($1,$2::date,$3::meal_type,'signed',$4) returning id`, uid, date, meal, signedAt).Scan(&id)
		signupCount++
		return id
	}

	type reviewKey struct{ uid int64; date, meal, dish string }
	reviewSeen := map[reviewKey]bool{}

	for d := -10; d <= 14; d++ {
		date := time.Now().AddDate(0, 0, d).Format("2006-01-02")
		base, _ := time.ParseInLocation("2006-01-02", date, time.Local)
		isPast := d < 0
		isToday := d == 0

		for mealI, meal := range []string{"breakfast", "lunch", "dinner"} {
			signedUIDs := []int64{}
			for _, e := range empList {
				uid := empIDs[e.no]
				if e.no == "E030" {
					continue // 禁用账号不产生业务数据
				}
				var p float64
				switch {
				case isPast:
					p = 0.55 + rng.Float64()*0.25 // 55%~80% 报名
				case isToday:
					if mealI < 2 {
						p = 0.6 // 早/午餐：昨天报的
					} else {
						p = 0.45 // 晚餐：还能报
					}
				case d <= 7:
					p = 0.4 + rng.Float64()*0.2
				default:
					p = 0.2
				}
				if !prob(p) {
					continue
				}
				signedAt := base.Add(-1 * time.Duration(int64(48+rint(10))) * time.Hour)
				id := insSignup(uid, date, meal, signedAt)
				signedUIDs = append(signedUIDs, uid)

				switch {
				case isPast:
					r := rng.Float64()
					switch {
					case r < 0.72: // 核销
						if _, err := conn.Exec(ctx, `update meal_signups set status='verified' where id=$1`, id); err == nil {
							by := adminIDs[rint(len(adminIDs))]
							at := base.Add(time.Duration(int64(deadlines[meal]-rint(45)-5)) * time.Minute)
							_, _ = conn.Exec(ctx, `insert into verifications (signup_id,verified_by,verified_at) values ($1,$2,$3)`, id, by, at)
							verifyCount++
						}
					case r < 0.85: // 未核销 → 违约
						noShowCount++
					default: // 取消（前一天取消）
						_, _ = conn.Exec(ctx, `update meal_signups set status='cancelled', cancelled_at=$2 where id=$1`,
							id, base.Add(-24*time.Hour))
						cancelCount++
					}
				case isToday && mealI < 2:
					// 早/午餐已过核销截止：一部分核销，其余推导违约
					if prob(0.6) {
						_, _ = conn.Exec(ctx, `update meal_signups set status='verified' where id=$1`, id)
						by := adminIDs[rint(len(adminIDs))]
						at := base.Add(time.Duration(int64(deadlines[meal])) * time.Hour)
						_, _ = conn.Exec(ctx, `insert into verifications (signup_id,verified_by,verified_at) values ($1,$2,$3)`, id, by, at)
						verifyCount++
					} else {
						noShowCount++
					}
				}
			}

			// 未报名就餐（walk_in）：每天随机 2~5 人
			if d <= 0 {
				n := 2 + rint(4)
				used := map[int64]bool{}
				for _, u := range signedUIDs {
					used[u] = true
				}
				for i := 0; i < n; i++ {
					e := empList[rint(len(empList)-1)] // 跳过禁用
					uid := empIDs[e.no]
					if used[uid] {
						continue
					}
					used[uid] = true
					_, err := conn.Exec(ctx, `insert into walk_in_records (user_id,menu_date,meal,registered_by,registered_at)
						values ($1,$2::date,$3::meal_type,$4,$5)`, uid, date, meal, adminIDs[0],
						base.Add(time.Duration(int64(11+rint(6))) * time.Hour))
					if err == nil {
						walkInCount++
					}
				}
			}

			// 点评：对过去/今天已核销的人，随机对一道当天该餐的菜评价
			if d <= 0 {
				for _, uid := range signedUIDs {
					if !prob(0.55) {
						continue
					}
					dishes := dayMenus[date][meal]
					if len(dishes) == 0 {
						continue
					}
					dish := pick(dishes)
					key := reviewKey{uid, date, meal, dish}
					if reviewSeen[key] {
						continue
					}
					reviewSeen[key] = true
					rating := 1
					if !prob(0.75) {
						rating = -1
					}
					at := base.Add(time.Duration(int64(deadlines[meal]+2)) * time.Hour)
					if _, err := conn.Exec(ctx, `insert into dish_reviews (user_id,menu_date,meal,dish_name,rating,created_at)
						values ($1,$2::date,$3::meal_type,$4,$5,$6)`, uid, date, meal, dish, rating, at); err == nil {
						reviewCount++
					}
				}
			}
		}
	}
	fmt.Printf("[4/5] 报餐 %d 条（核销 %d / 违约 %d / 取消 %d）· 未报名就餐 %d 条 · 点评 %d 条\n",
		signupCount, verifyCount, noShowCount, cancelCount, walkInCount, reviewCount)

	/* ---------- 5. 建议 ---------- */
	sugs := []struct {
		text string
		st   string
	}{
		{"希望午餐能加个水果，比如饭后一个橘子", "adopted"},
		{"麻辣香锅很受欢迎，能不能每周固定一天供应", "adopted"},
		{"早餐的包子有时候凉的，建议加保温措施", "pending"},
		{"想吃酸汤肥牛，求安排", "pending"},
		{"晚餐能不能提供一些轻食沙拉，减肥的同事也需要", "pending"},
		{"米饭有时候偏硬，建议控制好水量", "rejected"},
		{"希望增加清真窗口", "pending"},
		{"汤太咸了，清淡一点更好", "pending"},
		{"夜班同事希望能有夜宵供应", "rejected"},
		{"菜品很不错，感谢食堂师傅们！", "adopted"},
	}
	// 建议的提交人从员工里挑
	for i, s := range sugs {
		e := empList[i%len(empList)]
		uid := empIDs[e.no]
		at := time.Now().AddDate(0, 0, -(5 + i)).Add(time.Duration(int64(9+i)) * time.Hour)
		_, _ = conn.Exec(ctx, `insert into suggestions (user_id,content,status,created_at) values ($1,$2,$3,$4)`,
			uid, s.text, s.st, at)
	}
	fmt.Println("[5/5] 建议 10 条（待处理/已采纳/已拒绝 均有）")

	/* ---------- 汇总校验 ---------- */
	var u, mi, ms, v, w, dr, sg int64
	_ = conn.QueryRow(ctx, `select
		(select count(*) from users), (select count(*) from menu_items),
		(select count(*) from meal_signups), (select count(*) from verifications),
		(select count(*) from walk_in_records), (select count(*) from dish_reviews),
		(select count(*) from suggestions)`).Scan(&u, &mi, &ms, &v, &w, &dr, &sg)
	fmt.Printf("\n=== 库内总量：用户 %d · 菜品 %d · 报餐 %d · 核销 %d · 未报名就餐 %d · 点评 %d · 建议 %d ===\n",
		u, mi, ms, v, w, dr, sg)
	fmt.Println("登录账号：admin/admin123（超管，可能已被你改过密码）；canteen01、E001~E030 密码均为 123456")
}
