import type {
  Dataset,
  DatasetStats,
  PerformanceData,
  SearchResult,
  SystemHealth,
  SystemStats,
  UpsertResult,
  ActivityEvent,
} from './types'

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, { cache: 'no-store' })
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`)
  return res.json()
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `POST ${path} → ${res.status}`)
  }
  return res.json()
}

export const api = {
  // ── Datasets ─────────────────────────────────────────────
  listDatasets: (): Promise<{ datasets: Dataset[] }> =>
    get('/datasets'),

  createDataset: (name: string, source?: string): Promise<{ id: string; name: string }> =>
    post('/datasets', { name, source }),

  getDatasetStats: (id: string): Promise<DatasetStats> =>
    get(`/api/datasets/${id}/stats`),

  // ── Search ───────────────────────────────────────────────
  search: (
    datasetId: string,
    q: string,
    limit = 20,
    offset = 0,
    fuzziness = 1,
  ): Promise<SearchResult> =>
    get(
      `/datasets/${datasetId}/search?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}&fuzziness=${fuzziness}`,
    ),

  // ── Bulk write ───────────────────────────────────────────
  bulkUpsert: (
    datasetId: string,
    records: unknown[],
    syncToken?: string,
  ): Promise<UpsertResult> =>
    post(`/datasets/${datasetId}/records/bulk`, { records, sync_token: syncToken }),

  // ── System dashboard ─────────────────────────────────────
  getSystemStats: (): Promise<SystemStats> =>
    get('/api/system/stats'),

  getSystemHealth: (): Promise<SystemHealth> =>
    get('/api/system/health'),

  getActivity: (limit = 20): Promise<{ events: ActivityEvent[] }> =>
    get(`/api/activity?limit=${limit}`),

  getPerformance: (n = 1000): Promise<PerformanceData> =>
    get(`/api/performance?n=${n}`),
}
