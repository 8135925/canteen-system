// Package store 数据访问层：接口 + 类型定义。PostgreSQL 实现见 pg 包；内存实现供测试。
package store

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

// —— 领域类型 ——

type User struct {
	ID                 int64  `json:"id"`
	EmpNo              string `json:"emp_no"`
	Name               string `json:"name"`
	Department         string `json:"department"`
	PasswordHash       string `json:"-"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"must_change_password"`
}

type MenuItem struct {
	ID        int64    `json:"id"`
	MenuDate  string   `json:"menu_date"`
	Meal      string   `json:"meal"`
	DishName  string   `json:"dish_name"`
	TasteTags []string `json:"taste_tags"`
	Status    string   `json:"status"`
}

type MenuItemInput struct {
	MenuDate  string
	Meal      string
	DishName  string
	TasteTags []string
}

// DishWithReviews 菜谱展示项（含同名合并赞踩数）。
type DishWithReviews struct {
	MenuItem
	LikeCount    int `json:"like_count"`
	DislikeCount int `json:"dislike_count"`
}

type PublishFailure struct {
	MenuDate string `json:"menu_date"`
	Meal     string `json:"meal"`
	DishName string `json:"dish_name"`
	Reason   string `json:"reason"`
}

type VerifyRule struct {
	Meal     string `json:"meal"`
	Deadline string `json:"deadline"`
}

type Signup struct {
	ID          int64
	UserID      int64
	MenuDate    string
	Meal        string
	Status      string
	SignedAt    time.Time
	CancelledAt *time.Time
}

// SignupView 员工用餐记录（含推导违约状态）。
type SignupView struct {
	MenuDate    string     `json:"menu_date"`
	Meal        string     `json:"meal"`
	Status      string     `json:"status"` // signed/no_show/verified/cancelled（推导态）
	SignedAt    time.Time  `json:"signed_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}

type WalkIn struct {
	ID           int64
	UserID       int64
	MenuDate     string
	Meal         string
	RegisteredBy int64
	RegisteredAt time.Time
}

type Verification struct {
	ID         int64
	SignupID   int64
	VerifiedBy int64
	VerifiedAt time.Time
}

type Review struct {
	ID        int64
	UserID    int64
	MenuDate  string
	Meal      string
	DishName  string
	Rating    int // 1=赞 / -1=踩
	CreatedAt time.Time
}

type Suggestion struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// —— 统计行类型 ——

type MealCount struct {
	Meal  string `json:"meal"`
	Count int    `json:"count"`
}

type DeptCount struct {
	Department string `json:"department"`
	Count      int    `json:"count"`
}

type DailyActualRow struct {
	Date      string `json:"date"`
	Verified  int    `json:"verified"`
	WalkIn    int    `json:"walk_in"`
	Signups   int    `json:"signups"`
	Diff      int    `json:"diff"`
}

type ViolationRow struct {
	UserID      int64  `json:"user_id"`
	EmpNo       string `json:"emp_no"`
	Name        string `json:"name"`
	Department  string `json:"department"`
	NoShow      int    `json:"no_show"`
	WalkIn      int    `json:"walk_in"`
	Total       int    `json:"total"`
}

type DailyTrendRow struct {
	Date    string `json:"date"`
	Dining  int    `json:"dining"`
}

type DishRankRow struct {
	DishName     string `json:"dish_name"`
	LikeCount    int    `json:"like_count"`
	DislikeCount int    `json:"dislike_count"`
}

type WasteRateRow struct {
	Date       string  `json:"date"`
	NoShow     int     `json:"no_show"`
	Signups    int     `json:"signups"`
	WasteRate  float64 `json:"waste_rate"`
}

type DeptDiningRow struct {
	Department string `json:"department"`
	Dining     int    `json:"dining"`
}

type MealSignupCount struct {
	Meal      string `json:"meal"`
	Dishes    string `json:"dishes"`
	Count     int    `json:"count"`
}

// —— Store 接口 ——

type Store interface {
	Ping() error

	// users
	CreateUser(u *User) error
	GetUserByID(id int64) (*User, error)      // 未找到返回 ErrNotFound
	GetUserByEmpNo(empNo string) (*User, error)
	UpdateUserProfile(id int64, name, dept string) error
	UpdateUserStatus(id int64, status string) error
	UpdateUserPassword(id int64, hash string, mcp bool) error
	EmpNoExists(empNo string) (bool, error)
	ListUsers(role, keyword string, page, pageSize int) ([]User, int, error)

	// menu
	PublishMenu(items []MenuItemInput) ([]PublishFailure, error) // 冲突行跳过返回明细，其余提交
	ListMenuItems(from, to string) ([]DishWithReviews, error)
	GetMenuItem(id int64) (*MenuItem, error)
	SetMenuItemStatus(id int64, status string) error
	ExistsMenuForMeal(date, meal string) (bool, error)
	ListMenuRange(from, to string) ([]MenuItem, error)

	// verify rules
	ListVerifyRules() ([]VerifyRule, error)
	UpsertVerifyRule(meal, deadline string) error

	// signups
	InsertSignup(userID int64, date, meal string, now time.Time) (*Signup, error)
	CancelLatestSignup(userID int64, date, meal string, now time.Time) error // 最新行置 cancelled
	GetLatestSignup(userID int64, date, meal string) (*Signup, error)
	ListMySignups(userID int64, from, to string) ([]Signup, error) // 每餐次最新一行
	ListWalkInsOfUser(userID int64, from, to string) ([]WalkIn, error)

	// verify / walk-in
	InsertVerification(signupID, verifiedBy int64, now time.Time) error
	InsertWalkIn(userID int64, date, meal string, by int64, now time.Time) error // 冲突返回 ErrConflict
	HasEaten(userID int64, date, meal string) (bool, error)

	// reviews
	InsertReview(userID int64, date, meal, dish string, rating int, now time.Time) error

	// suggestions
	InsertSuggestion(userID int64, content string, now time.Time) (*Suggestion, error)
	GetSuggestion(id int64) (*Suggestion, error)
	ListMySuggestions(userID int64) ([]Suggestion, error)
	ListSuggestions(status string) ([]Suggestion, error)
	UpdateSuggestionStatus(id int64, status string) error

	// stats
	CountSignupsByMeal(date string) ([]MealCount, error)
	SignupByDepartment(date string) ([]DeptCount, error)
	DailyActual(from, to string) ([]DailyActualRow, error)
	ViolationRank(from, to string) ([]ViolationRow, error)
	DailyTrend(from, to string) ([]DailyTrendRow, error)
	DishRanking(from, to string) ([]DishRankRow, error)
	WasteRate(from, to string) ([]WasteRateRow, error)
	DeptDining(from, to string) ([]DeptDiningRow, error)
	DishSatisfaction() (likeCount, dislikeCount int64, err error)
	AttendanceDetail(date, dept, keyword string, page, pageSize int) ([]AttendanceRow, int64, error)
	DBNow() (time.Time, error)
}

// AttendanceRow 用餐明细行：某天某人在三餐上的状态。
// 每餐取值：verified=报名已核销 / no_show=报名未到 / signed=已报名未到核销时点 /
// cancelled=已取消 / walk_in=未报名就餐 / ""=未报名也未就餐。
type AttendanceRow struct {
	UserID     int64  `json:"user_id"`
	EmpNo      string `json:"emp_no"`
	Name       string `json:"name"`
	Department string `json:"department"`
	UserStatus string `json:"user_status"`
	Breakfast  string `json:"breakfast"`
	Lunch      string `json:"lunch"`
	Dinner     string `json:"dinner"`
}

// ErrConflict 唯一约束冲突（幂等兜底场景）。
var ErrConflict = errors.New("conflict")
