// 共享 API 客户端：统一 request 封装（自动带 JWT、401 跳登录、MUST_CHANGE_PASSWORD 跳改密、统一错误提示）。

export const API_BASE = '/api/v1'

export interface ApiEnvelope<T = any> {
  code: string
  msg: string
  data: T
}

export interface LoginUser {
  id: number
  emp_no: string
  name: string
  department: string
  role: 'employee' | 'canteen_admin' | 'super_admin'
  must_change_password: boolean
}

let onUnauthorized: () => void = () => {}
let onMustChangePassword: () => void = () => {}
let onErrorToast: (msg: string) => void = () => {}

export function configureClient(opts: {
  onUnauthorized?: () => void
  onMustChangePassword?: () => void
  onErrorToast?: (msg: string) => void
}) {
  if (opts.onUnauthorized) onUnauthorized = opts.onUnauthorized
  if (opts.onMustChangePassword) onMustChangePassword = opts.onMustChangePassword
  if (opts.onErrorToast) onErrorToast = opts.onErrorToast
}

export function getToken(): string {
  // remember token 存 localStorage（30 天），普通 token 存 sessionStorage（12h）
  return localStorage.getItem('canteen_token') || sessionStorage.getItem('canteen_token') || ''
}

export function setToken(token: string, remember: boolean) {
  clearToken()
  if (remember) localStorage.setItem('canteen_token', token)
  else sessionStorage.setItem('canteen_token', token)
}

export function clearToken() {
  localStorage.removeItem('canteen_token')
  sessionStorage.removeItem('canteen_token')
}

export class ApiError extends Error {
  code: string
  constructor(code: string, msg: string) {
    super(msg)
    this.code = code
  }
}

export async function request<T = any>(
  method: string,
  path: string,
  opts: { body?: any; query?: Record<string, string>; skipAuthRedirect?: boolean } = {}
): Promise<T> {
  const url = new URL(API_BASE + path, window.location.origin)
  if (opts.query) {
    for (const [k, v] of Object.entries(opts.query)) {
      if (v !== undefined && v !== null && v !== '') url.searchParams.set(k, v)
    }
  }
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  const resp = await fetch(url.toString(), {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  const env = (await resp.json()) as ApiEnvelope<T>
  if (env.code !== 'OK') {
    // 登录接口自身的 401（密码错误）应展示真实错误，不触发"请先登录"
    if (resp.status === 401 && !opts.skipAuthRedirect) {
      clearToken()
      onUnauthorized()
    } else if (env.code === 'MUST_CHANGE_PASSWORD') {
      onMustChangePassword()
    } else {
      onErrorToast(env.msg || '请求失败')
    }
    throw new ApiError(env.code, env.msg)
  }
  return env.data
}

// —— 便捷方法 ——
export const get = <T = any>(path: string, query?: Record<string, string>) => request<T>('GET', path, { query })
export const post = <T = any>(path: string, body?: any) => request<T>('POST', path, { body })
export const put = <T = any>(path: string, body?: any) => request<T>('PUT', path, { body })
export const patch = <T = any>(path: string, body?: any) => request<T>('PATCH', path, { body })
export const del = <T = any>(path: string, query?: Record<string, string>) => request<T>('DELETE', path, { query })

// 上传（multipart）
export async function upload(path: string, file: File): Promise<any> {
  const fd = new FormData()
  fd.append('file', file)
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  const resp = await fetch(API_BASE + path, { method: 'POST', headers, body: fd })
  const env = (await resp.json()) as ApiEnvelope
  if (env.code !== 'OK') {
    onErrorToast(env.msg || '上传失败')
    throw new ApiError(env.code, env.msg)
  }
  return env.data
}

// —— 业务类型 ——

export interface Dish {
  id: number
  menu_date: string
  meal: string
  dish_name: string
  taste_tags: string[]
  status: 'normal' | 'off_shelf'
  like_count: number
  dislike_count: number
}

export interface MealCount { meal: string; count: number }
export interface DeptCount { department: string; count: number }
export interface DailyActualRow { date: string; verified: number; walk_in: number; signups: number; diff: number }
export interface ViolationRow { user_id: number; emp_no: string; name: string; department: string; no_show: number; walk_in: number; total: number }
export interface DailyTrendRow { date: string; dining: number }
export interface DishRankRow { dish_name: string; like_count: number; dislike_count: number }
export interface WasteRateRow { date: string; no_show: number; signups: number; waste_rate: number }
export interface DeptDiningRow { department: string; dining: number }
export interface Suggestion { id: number; user_id: number; user_name: string; content: string; status: string; created_at: string }
export interface SignupView { menu_date: string; meal: string; status: string; signed_at: string; cancelled_at?: string }
