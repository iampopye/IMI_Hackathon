'use client'
import { useState } from 'react'
import { X } from 'lucide-react'
import { api } from '@/lib/api'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface NewDatasetModalProps {
  onClose: () => void
  onCreated: (id: string, name: string) => void
}

export function NewDatasetModal({ onClose, onCreated }: NewDatasetModalProps) {
  const [name, setName] = useState('')
  const [source, setSource] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const create = async () => {
    if (!name.trim()) return
    setLoading(true)
    setError(null)
    try {
      const res = await api.createDataset(name.trim(), source.trim() || undefined)
      onCreated(res.id, res.name)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create dataset')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-card shadow-xl w-full max-w-md border border-warm-border">
        <div className="flex items-center justify-between px-5 py-4 border-b border-warm-border">
          <h2 className="font-display font-semibold text-ink">New Dataset</h2>
          <button onClick={onClose} className="text-ink-muted hover:text-ink">
            <X size={16} />
          </button>
        </div>

        <div className="px-5 py-4 flex flex-col gap-4">
          <div>
            <label className="text-xs font-medium text-ink-secondary mb-1.5 block">Name *</label>
            <Input
              placeholder="products_catalog"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && create()}
              autoFocus
            />
          </div>
          <div>
            <label className="text-xs font-medium text-ink-secondary mb-1.5 block">Source</label>
            <Input
              placeholder="erp_system"
              value={source}
              onChange={(e) => setSource(e.target.value)}
            />
          </div>
          {error && <p className="text-xs text-danger">{error}</p>}
        </div>

        <div className="flex justify-end gap-2 px-5 py-3 border-t border-warm-border">
          <Button variant="ghost" size="sm" onClick={onClose}>Cancel</Button>
          <Button size="sm" onClick={create} disabled={!name.trim() || loading}>
            {loading ? 'Creating…' : 'Create'}
          </Button>
        </div>
      </div>
    </div>
  )
}
