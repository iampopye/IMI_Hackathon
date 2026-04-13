// TypeScript types matching Go API structs

export interface Hit {
  id: string
  name: string
  score: number
  value?: Record<string, unknown>
}

export interface SearchResult {
  hits: Hit[]
  total: number
  engine: string
  took_ns: number
}

export interface Dataset {
  id: string
  name: string
  source: string
  record_count: number
  stability_score: number
  state: string
}

export interface SystemStats {
  total_records: number
  searches_today: number
  cache_hit_rate: number
  avg_latency_ms: number
  outbox_pending: number
}

export interface ServiceStatus {
  name: string
  ok: boolean
  latency_ms: number
}

export interface SystemHealth {
  services: ServiceStatus[]
  all_ok: boolean
}

export interface QueryLogEntry {
  time: string
  dataset_id: string
  term: string
  engine: string
  latency_ms: number
  hits: number
  cache_hit: boolean
}

export interface PerformanceData {
  p50: number
  p95: number
  p99: number
  cache_hit_rate: number
  queries: QueryLogEntry[]
}

export interface SyncEntry {
  time: string
  type: string
  inserted: number
  skipped: number
  failed: number
}

export interface DatasetStats {
  sync_history: SyncEntry[]
  value_fields: string[]
}

export interface ActivityEvent {
  time: string
  type: string
  dataset: string
  message: string
  engine: string
  latency_ms: number
}

export interface UpsertResult {
  inserted: number
  updated: number
  skipped: number
  failed: number
  total: number
  duration_ms?: number
}
