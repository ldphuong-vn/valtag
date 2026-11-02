'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import type { ScanResult } from '@/types/api'
import { ScanLine, AlertCircle, Info } from 'lucide-react'
import { cn } from '@/lib/utils'

export default function ScansPage() {
  const [projectId, setProjectId] = useState<string | null>(null)
  const [scans, setScans] = useState<ScanResult[]>([])
  const [loading, setLoading] = useState(true)
  const [showInfo, setShowInfo] = useState(false)

  useEffect(() => {
    const pid = typeof window !== 'undefined' ? localStorage.getItem('valt_current_project') : null
    setProjectId(pid)
    if (pid) {
      api.scans.list(pid)
        .then(r => setScans(r.scans ?? []))
        .catch(() => setScans([]))
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
    }
  }, [])

  if (!projectId) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
        <AlertCircle className="w-8 h-8" />
        <p>No project selected. <a href="/projects" className="text-primary underline">Select a project</a> first.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Secret Scanner</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Scan your codebase for hardcoded secrets and import them into valtag.
          </p>
        </div>
        <button
          onClick={() => setShowInfo(v => !v)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90"
        >
          <ScanLine className="w-4 h-4" />
          How to Scan
        </button>
      </div>

      {showInfo && (
        <div className="flex gap-3 p-4 rounded-lg border bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800 text-sm">
          <Info className="w-5 h-5 text-blue-600 dark:text-blue-400 shrink-0 mt-0.5" />
          <div className="space-y-1 text-blue-800 dark:text-blue-200">
            <p className="font-medium">How to Scan using the valtag MCP tool</p>
            <p>In your AI agent (Claude, Cursor, etc.), use the <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">scan_secrets</code> tool:</p>
            <code className="block bg-blue-100 dark:bg-blue-900 px-2 py-1 rounded mt-1 font-mono text-xs">
              {'scan_secrets({ path: "/path/to/project", recursive: true })'}
            </code>
            <p className="mt-1">Then use <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">store_secret</code> to import findings into valtag.</p>
          </div>
        </div>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading scans…</p>
      ) : scans.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-40 gap-2 text-muted-foreground border rounded-lg">
          <ScanLine className="w-8 h-8" />
          <p className="text-sm">No scans yet. Run <code className="bg-muted px-1 rounded">scan_secrets</code> via the MCP tool.</p>
        </div>
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Path</th>
                <th className="text-left px-4 py-2 font-medium">Findings</th>
                <th className="text-left px-4 py-2 font-medium">Status</th>
                <th className="text-left px-4 py-2 font-medium">Date</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y">
              {scans.map(scan => (
                <tr key={scan.id} className="hover:bg-muted/30">
                  <td className="px-4 py-2 font-mono text-xs max-w-xs truncate" title={scan.scan_path}>
                    {scan.scan_path}
                  </td>
                  <td className="px-4 py-2">
                    <span className={cn(
                      'px-2 py-0.5 rounded-full text-xs font-medium',
                      scan.findings_count > 0
                        ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                        : 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                    )}>
                      {scan.findings_count} {scan.findings_count === 1 ? 'finding' : 'findings'}
                    </span>
                  </td>
                  <td className="px-4 py-2">
                    <span className={cn(
                      'px-2 py-0.5 rounded-full text-xs',
                      scan.status === 'completed' ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' : 'bg-muted text-muted-foreground'
                    )}>
                      {scan.status}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">
                    {new Date(scan.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <a href={`/scans/${scan.id}`} className="text-primary text-xs hover:underline">
                      View findings
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
