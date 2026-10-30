// Package pg Store 的 PostgreSQL（pgx/v5）实现。
// 连接 Supabase Session Pooler（:5432, TLS）；查询全部参数化，无拼接。
package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"canteen/server/internal/migrate"
	"canteen/server/internal/store"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 8
	cfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping() error { return s.pool.Ping(context.Background()) }

// Migrate 执行内嵌 SQL 迁移（AUTO_MIGRATE=1 时由 main 调用）。
func (s *Store) Migrate(ctx context.Context) error { return migrate.Up(ctx, s.pool) }

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

// —— users ——

const userCols = `id, emp_no, name, department, password_hash, role::text, status, must_change_password`

func scanUser(row pgx.Row) (*store.User, error) {
	var u store.User
	err := row.Scan(&u.ID, &u.EmpNo, &u.Name, &u.Department, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword)
	if err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (s *Store) CreateUser(u *store.User) error {
	return s.pool.QueryRow(context.Background(),
		`insert into users (emp_no, name, department, password_hash, role, must_change_password)
		 values ($1,$2,$3,$4,$5,$6) returning `+userCols,
		u.EmpNo, u.Name, u.Department, u.PasswordHash, u.Role, u.MustChangePassword,
	).Scan(&u.ID, &u.EmpNo, &u.Name, &u.Department, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword)
}

func (s *Store) GetUserByID(id int64) (*store.User, error) {
	return scanUser(s.pool.QueryRow(context.Background(), `select `+userCols+` from users where id=$1`, id))
}

func (s *Store) GetUserByEmpNo(empNo string) (*store.User, error) {
	return scanUser(s.pool.QueryRow(context.Background(), `select `+userCols+` from users where emp_no=$1`, empNo))
}

func (s *Store) UpdateUserProfile(id int64, name, dept string) error {
	_, err := s.pool.Exec(context.Background(),
		`update users set name=$2, department=$3, updated_at=now() where id=$1`, id, name, dept)
	return err
}

func (s *Store) UpdateUserStatus(id int64, status string) error {
	_, err := s.pool.Exec(context.Background(),
		`update users set status=$2, updated_at=now() where id=$1`, id, status)
	return err
}

func (s *Store) UpdateUserPassword(id int64, hash string, mcp bool) error {
	_, err := s.pool.Exec(context.Background(),
		`update users set password_hash=$2, must_change_password=$3, updated_at=now() where id=$1`, id, hash, mcp)
	return err
}

func (s *Store) EmpNoExists(empNo string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(context.Background(), `select exists(select 1 from users where emp_no=$1)`, empNo).Scan(&exists)
	return exists, err
}

func (s *Store) ListUsers(role, keyword string, page, pageSize int) ([]store.User, int, error) {
	ctx := context.Background()
	cond := `where role::text in ('employee','canteen_admin')`
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if role == "employee" || role == "canteen_admin" {
		cond = `where role::text = ` + arg(role)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		cond += ` and (emp_no like ` + arg(kw) + ` or name like ` + arg(kw) + `)`
	}
	var total int
	if err := s.pool.QueryRow(ctx, `select count(*) from users `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx,
		`select `+userCols+` from users `+cond+fmt.Sprintf(` order by id limit $%d offset $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []store.User
	for rows.Next() {
		var u store.User
		if err := rows.Scan(&u.ID, &u.EmpNo, &u.Name, &u.Department, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// —— menu ——

func (s *Store) PublishMenu(items []store.MenuItemInput) ([]store.PublishFailure, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var failures []store.PublishFailure
	for _, it := range items {
		tags := it.TasteTags
		if tags == nil {
			tags = []string{}
		}
		var id int64
		err := tx.QueryRow(ctx,
			`insert into menu_items (menu_date, meal, dish_name, taste_tags) values ($1,$2,$3,$4) on conflict do nothing returning id`,
			it.MenuDate, it.Meal, it.DishName, tags).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			failures = append(failures, store.PublishFailure{MenuDate: it.MenuDate, Meal: it.Meal, DishName: it.DishName, Reason: "同一日期同一餐次菜品名重复"})
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return failures, tx.Commit(ctx)
}

const menuCols = `id, menu_date::text, meal::text, dish_name, taste_tags, status`

func (s *Store) ListMenuItems(from, to string) ([]store.DishWithReviews, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select m.id, m.menu_date::text, m.meal::text, m.dish_name, m.taste_tags, m.status,
		       coalesce(r.like_count,0), coalesce(r.dislike_count,0)
		from menu_items m
		left join (
		  select menu_date, meal, dish_name,
		         sum(case when rating=1 then 1 else 0 end) as like_count,
		         sum(case when rating=-1 then 1 else 0 end) as dislike_count
		  from dish_reviews group by menu_date, meal, dish_name
		) r on r.menu_date=m.menu_date and r.meal=m.meal and r.dish_name=m.dish_name
		where m.menu_date between $1 and $2
		order by m.menu_date, m.meal, m.id`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.DishWithReviews
	for rows.Next() {
		var d store.DishWithReviews
		if err := rows.Scan(&d.ID, &d.MenuDate, &d.Meal, &d.DishName, &d.TasteTags, &d.Status, &d.LikeCount, &d.DislikeCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetMenuItem(id int64) (*store.MenuItem, error) {
	var m store.MenuItem
	err := s.pool.QueryRow(context.Background(),
		`select `+menuCols+` from menu_items where id=$1`, id).
		Scan(&m.ID, &m.MenuDate, &m.Meal, &m.DishName, &m.TasteTags, &m.Status)
	if err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (s *Store) SetMenuItemStatus(id int64, status string) error {
	_, err := s.pool.Exec(context.Background(),
		`update menu_items set status=$2, updated_at=now() where id=$1`, id, status)
	return err
}

func (s *Store) ExistsMenuForMeal(date, meal string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(context.Background(),
		`select exists(select 1 from menu_items where menu_date=$1 and meal=$2)`, date, meal).Scan(&exists)
	return exists, err
}

func (s *Store) ListMenuRange(from, to string) ([]store.MenuItem, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `select `+menuCols+` from menu_items where menu_date between $1 and $2 order by id`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.MenuItem
	for rows.Next() {
		var m store.MenuItem
		if err := rows.Scan(&m.ID, &m.MenuDate, &m.Meal, &m.DishName, &m.TasteTags, &m.Status); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// —— verify rules ——

func (s *Store) ListVerifyRules() ([]store.VerifyRule, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `select meal::text, verify_deadline::text from verify_rules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.VerifyRule
	for rows.Next() {
		var r store.VerifyRule
		if err := rows.Scan(&r.Meal, &r.Deadline); err != nil {
			return nil, err
		}
		// time::text 可能带秒，统一 HH:MM:SS
		r.Deadline = normClock(r.Deadline)
		out = append(out, r)
	}
	return out, rows.Err()
}

func normClock(s string) string {
	parts := strings.Split(s, ":")
	for len(parts) < 3 {
		parts = append(parts, "00")
	}
	for i := range parts {
		if len(parts[i]) == 1 {
			parts[i] = "0" + parts[i]
		}
	}
	return strings.Join(parts, ":")
}

func (s *Store) UpsertVerifyRule(meal, deadline string) error {
	_, err := s.pool.Exec(context.Background(),
		`insert into verify_rules (meal, verify_deadline) values ($1,$2::time)
		 on conflict (meal) do update set verify_deadline=excluded.verify_deadline, updated_at=now()`, meal, deadline)
	return err
}

// —— signups ——

const signupCols = `id, user_id, menu_date::text, meal::text, status, signed_at, cancelled_at`

func scanSignup(row pgx.Row) (*store.Signup, error) {
	var r store.Signup
	err := row.Scan(&r.ID, &r.UserID, &r.MenuDate, &r.Meal, &r.Status, &r.SignedAt, &r.CancelledAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &r, nil
}

func (s *Store) InsertSignup(userID int64, date, meal string, now time.Time) (*store.Signup, error) {
	return scanSignup(s.pool.QueryRow(context.Background(),
		`insert into meal_signups (user_id, menu_date, meal, status, signed_at) values ($1,$2,$3,'signed',$4) returning `+signupCols,
		userID, date, meal, now))
}

func (s *Store) CancelLatestSignup(userID int64, date, meal string, now time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		update meal_signups set status='cancelled', cancelled_at=$4
		where id = (
		  select id from meal_signups
		  where user_id=$1 and menu_date=$2 and meal=$3 and status='signed'
		  order by signed_at desc, id desc limit 1
		)`, userID, date, meal, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) GetLatestSignup(userID int64, date, meal string) (*store.Signup, error) {
	return scanSignup(s.pool.QueryRow(context.Background(), `
		select `+signupCols+` from meal_signups
		where user_id=$1 and menu_date=$2 and meal=$3
		order by signed_at desc, id desc limit 1`, userID, date, meal))
}

func (s *Store) ListMySignups(userID int64, from, to string) ([]store.Signup, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select distinct on (menu_date, meal) `+signupCols+`
		from meal_signups
		where user_id=$1 and menu_date between $2 and $3
		order by menu_date, meal, signed_at desc, id desc`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Signup
	for rows.Next() {
		var r store.Signup
		if err := rows.Scan(&r.ID, &r.UserID, &r.MenuDate, &r.Meal, &r.Status, &r.SignedAt, &r.CancelledAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListWalkInsOfUser(userID int64, from, to string) ([]store.WalkIn, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select id, user_id, menu_date::text, meal::text, registered_by, registered_at
		from walk_in_records where user_id=$1 and menu_date between $2 and $3 order by id`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WalkIn
	for rows.Next() {
		var wk store.WalkIn
		if err := rows.Scan(&wk.ID, &wk.UserID, &wk.MenuDate, &wk.Meal, &wk.RegisteredBy, &wk.RegisteredAt); err != nil {
			return nil, err
		}
		out = append(out, wk)
	}
	return out, rows.Err()
}

// —— verify / walk-in ——

func (s *Store) InsertVerification(signupID, verifiedBy int64, now time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `update meal_signups set status='verified' where id=$1 and status='signed'`, signupID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrConflict // 非 signed（已 verified 或已取消）——调用方此前已判定
	}
	if _, err := tx.Exec(ctx, `insert into verifications (signup_id, verified_by, verified_at) values ($1,$2,$3)`, signupID, verifiedBy, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) InsertWalkIn(userID int64, date, meal string, by int64, now time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `
		insert into walk_in_records (user_id, menu_date, meal, registered_by, registered_at)
		values ($1,$2,$3,$4,$5)
		on conflict (user_id, menu_date, meal) do nothing
		returning id`, userID, date, meal, by, now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrConflict
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) HasEaten(userID int64, date, meal string) (bool, error) {
	ctx := context.Background()
	var verified bool
	err := s.pool.QueryRow(ctx, `
		select exists(
		  select 1 from meal_signups
		  where user_id=$1 and menu_date=$2 and meal=$3 and status='verified'
		)`, userID, date, meal).Scan(&verified)
	if err != nil {
		return false, err
	}
	if verified {
		return true, nil
	}
	var walked bool
	err = s.pool.QueryRow(ctx, `
		select exists(select 1 from walk_in_records where user_id=$1 and menu_date=$2 and meal=$3)`,
		userID, date, meal).Scan(&walked)
	return walked, err
}

// —— reviews ——

func (s *Store) InsertReview(userID int64, date, meal, dish string, rating int, now time.Time) error {
	_, err := s.pool.Exec(context.Background(),
		`insert into dish_reviews (user_id, menu_date, meal, dish_name, rating, created_at) values ($1,$2,$3,$4,$5,$6)`,
		userID, date, meal, dish, rating, now)
	if err != nil && strings.Contains(err.Error(), "duplicate key") {
		return store.ErrConflict
	}
	return err
}

// —— suggestions ——

const suggestionCols = `s.id, s.user_id, coalesce(u.name,''), s.content, s.status, s.created_at`

func scanSuggestion(row pgx.Row) (*store.Suggestion, error) {
	var r store.Suggestion
	err := row.Scan(&r.ID, &r.UserID, &r.UserName, &r.Content, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &r, nil
}

func (s *Store) InsertSuggestion(userID int64, content string, now time.Time) (*store.Suggestion, error) {
	return scanSuggestion(s.pool.QueryRow(context.Background(),
		`insert into suggestions (user_id, content, created_at) values ($1,$2,$3) returning id, user_id, '', content, status, created_at`,
		userID, content, now))
}

func (s *Store) GetSuggestion(id int64) (*store.Suggestion, error) {
	return scanSuggestion(s.pool.QueryRow(context.Background(),
		`select `+suggestionCols+` from suggestions s left join users u on u.id=s.user_id where s.id=$1`, id))
}

func (s *Store) ListMySuggestions(userID int64) ([]store.Suggestion, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx,
		`select `+suggestionCols+` from suggestions s left join users u on u.id=s.user_id where s.user_id=$1 order by s.id desc`, userID)
	return collectSuggestions(rows, err)
}

func (s *Store) ListSuggestions(status string) ([]store.Suggestion, error) {
	ctx := context.Background()
	q := `select ` + suggestionCols + ` from suggestions s left join users u on u.id=s.user_id`
	args := []any{}
	if status != "" {
		q += ` where s.status=$1`
		args = append(args, status)
	}
	q += ` order by s.id desc`
	rows, err := s.pool.Query(ctx, q, args...)
	return collectSuggestions(rows, err)
}

func collectSuggestions(rows pgx.Rows, err error) ([]store.Suggestion, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Suggestion
	for rows.Next() {
		var r store.Suggestion
		if err := rows.Scan(&r.ID, &r.UserID, &r.UserName, &r.Content, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpdateSuggestionStatus(id int64, status string) error {
	_, err := s.pool.Exec(context.Background(), `update suggestions set status=$2 where id=$1`, id, status)
	return err
}

// —— stats ——（口径对齐 sheji.md §5.3：以每 (user,date,meal) 最新行为准）

func (s *Store) CountSignupsByMeal(date string) ([]store.MealCount, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select meal::text, count(*) from (
		  select distinct on (user_id, meal) user_id, meal::text, status
		  from meal_signups where menu_date=$1
		  order by user_id, meal, signed_at desc, id desc
		) t where status in ('signed','verified')
		group by meal`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var m string
		var c int
		if err := rows.Scan(&m, &c); err != nil {
			return nil, err
		}
		counts[m] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []store.MealCount
	for _, m := range []string{"breakfast", "lunch", "dinner"} {
		out = append(out, store.MealCount{Meal: m, Count: counts[m]})
	}
	return out, nil
}

func (s *Store) SignupByDepartment(date string) ([]store.DeptCount, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select coalesce(nullif(u.department,''),'未填写') as dept, count(*)
		from (
		  select distinct on (user_id, meal) user_id, status
		  from meal_signups where menu_date=$1
		  order by user_id, meal, signed_at desc, id desc
		) t join users u on u.id=t.user_id
		where t.status in ('signed','verified')
		group by dept order by dept`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeptCounts(rows, nil)
}

func scanDeptCounts(rows pgx.Rows, err error) ([]store.DeptCount, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.DeptCount
	for rows.Next() {
		var r store.DeptCount
		if err := rows.Scan(&r.Department, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DailyActual(from, to string) ([]store.DailyActualRow, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		with d as (select generate_series($1::date, $2::date, interval '1 day')::date as date),
		verified as (
		  select v.verified_at::date as date, count(*) c
		  from verifications v join meal_signups ms on ms.id=v.signup_id
		  where ms.menu_date between $1 and $2 group by 1
		),
		walkin as (
		  select registered_at::date as date, count(*) c
		  from walk_in_records where menu_date between $1 and $2 group by 1
		),
		signups as (
		  select menu_date as date, count(*) c from (
		    select distinct on (user_id, menu_date, meal) user_id, menu_date, meal, status
		    from meal_signups where menu_date between $1 and $2
		    order by user_id, menu_date, meal, signed_at desc, id desc
		  ) t where status in ('signed','verified') group by 1
		)
		select d.date::text,
		       coalesce(v.c,0), coalesce(w.c,0), coalesce(s.c,0),
		       coalesce(v.c,0)+coalesce(w.c,0)-coalesce(s.c,0)
		from d
		left join verified v on v.date=d.date
		left join walkin w on w.date=d.date
		left join signups s on s.date=d.date
		order by d.date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.DailyActualRow
	for rows.Next() {
		var r store.DailyActualRow
		if err := rows.Scan(&r.Date, &r.Verified, &r.WalkIn, &r.Signups, &r.Diff); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ViolationRank 违约名单：推导违约（最新行 signed 且 已过该餐核销截止）+ walk_in，按用户聚合。
// 核销截止取 verify_rules 覆盖值，缺省用默认常量（SQL 内 coalesce）。
func (s *Store) ViolationRank(from, to string) ([]store.ViolationRow, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		with rules as (
		  select meal::text as meal, verify_deadline::text as dl from verify_rules
		),
		latest as (
		  select distinct on (user_id, menu_date, meal)
		         user_id, menu_date, meal::text as meal, status
		  from meal_signups
		  where menu_date between $1 and $2
		  order by user_id, menu_date, meal, signed_at desc, id desc
		),
		no_show as (
		  select l.user_id, count(*)::int as c
		  from latest l
		  left join rules r on r.meal = l.meal
		  where l.status = 'signed'
		    and l.menu_date <= (now() at time zone 'Asia/Shanghai')::date
		    and (now() at time zone 'Asia/Shanghai')::time >
		        coalesce(r.dl::time,
		          case l.meal when 'breakfast' then time '09:00'
		                      when 'lunch'     then time '13:30'
		                      else                  time '19:00' end)
		  group by l.user_id
		),
		walkins as (
		  select user_id, count(*)::int as c
		  from walk_in_records
		  where menu_date between $1 and $2
		  group by user_id
		),
		agg as (
		  select coalesce(n.user_id, w.user_id) as uid,
		         coalesce(n.c, 0) as no_show,
		         coalesce(w.c, 0) as walkin
		  from no_show n
		  full outer join walkins w on w.user_id = n.user_id
		)
		select a.uid, u.emp_no, u.name,
		       coalesce(nullif(u.department,''),'未填写') as dept,
		       a.no_show, a.walkin, a.no_show + a.walkin as total
		from agg a
		join users u on u.id = a.uid
		order by total desc, u.emp_no`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.ViolationRow
	for rows.Next() {
		var r store.ViolationRow
		if err := rows.Scan(&r.UserID, &r.EmpNo, &r.Name, &r.Department, &r.NoShow, &r.WalkIn, &r.Total); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DailyTrend(from, to string) ([]store.DailyTrendRow, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		with d as (select generate_series($1::date, $2::date, interval '1 day')::date as date),
		verified as (
		  select v.verified_at::date as date, count(*) c
		  from verifications v join meal_signups ms on ms.id=v.signup_id
		  where ms.menu_date between $1 and $2 group by 1
		),
		walkin as (
		  select registered_at::date as date, count(*) c
		  from walk_in_records where menu_date between $1 and $2 group by 1
		)
		select d.date::text, coalesce(v.c,0)+coalesce(w.c,0)
		from d left join verified v on v.date=d.date left join walkin w on w.date=d.date
		order by d.date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.DailyTrendRow
	for rows.Next() {
		var r store.DailyTrendRow
		if err := rows.Scan(&r.Date, &r.Dining); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DishRanking(from, to string) ([]store.DishRankRow, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select dish_name,
		       sum(case when rating=1 then 1 else 0 end) as like_count,
		       sum(case when rating=-1 then 1 else 0 end) as dislike_count
		from dish_reviews where menu_date between $1 and $2
		group by dish_name order by like_count desc, dish_name`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.DishRankRow
	for rows.Next() {
		var r store.DishRankRow
		if err := rows.Scan(&r.DishName, &r.LikeCount, &r.DislikeCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) WasteRate(from, to string) ([]store.WasteRateRow, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		with d as (select generate_series($1::date, $2::date, interval '1 day')::date as date),
		latest as (
		  select distinct on (user_id, menu_date, meal) user_id, menu_date, meal::text as meal, status
		  from meal_signups where menu_date between $1 and $2
		  order by user_id, menu_date, meal, signed_at desc, id desc
		),
		rules as (select meal::text as meal, verify_deadline::text as dl from verify_rules),
		no_show as (
		  select l.menu_date as date, count(*) c
		  from latest l left join rules r on r.meal=l.meal
		  where l.status='signed'
		    and (now() at time zone 'Asia/Shanghai')::time > coalesce(r.dl::time, (case l.meal
		        when 'breakfast' then '09:00' when 'lunch' then '13:30' else '19:00' end)::time)
		    and l.menu_date <= (now() at time zone 'Asia/Shanghai')::date
		  group by 1
		),
		valid as (
		  select menu_date as date, count(*) c from latest
		  where status in ('signed','verified') group by 1
		)
		select d.date::text, coalesce(n.c,0), coalesce(v.c,0),
		       case when coalesce(v.c,0)>0 then round(coalesce(n.c,0)::numeric / v.c, 4)::float8 else 0 end
		from d left join no_show n on n.date=d.date left join valid v on v.date=d.date
		order by d.date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WasteRateRow
	for rows.Next() {
		var r store.WasteRateRow
		if err := rows.Scan(&r.Date, &r.NoShow, &r.Signups, &r.WasteRate); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeptDining(from, to string) ([]store.DeptDiningRow, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		select coalesce(nullif(u.department,''),'未填写') as dept, count(*) c
		from users u
		join (
		  select ms.user_id
		  from verifications v join meal_signups ms on ms.id = v.signup_id
		  where ms.menu_date between $1 and $2
		  union all
		  select w.user_id from walk_in_records w
		  where w.menu_date between $1 and $2
		) acts on acts.user_id = u.id
		group by dept order by dept`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	depts, err := scanDeptCounts(rows, nil)
	if err != nil {
		return nil, err
	}
	out := make([]store.DeptDiningRow, 0, len(depts))
	for _, d := range depts {
		out = append(out, store.DeptDiningRow{Department: d.Department, Dining: d.Count})
	}
	return out, nil
}
