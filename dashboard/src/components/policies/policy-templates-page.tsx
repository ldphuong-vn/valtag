'use client'

import { useEffect, useState } from 'react'
import { Copy, History, Plus } from 'lucide-react'
import { api } from '@/lib/api-client'
import type { PolicyParameters, PolicyTemplate, PolicyTemplateVersion, ProjectMembership } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { defaultPolicy } from './policy-helpers'
import { PolicyTemplateEditorDialog } from './policy-template-editor-dialog'
import { PolicyTemplateHistoryDialog } from './policy-template-history-dialog'

type Mode = 'create' | 'edit' | 'clone'

interface Props {
  projectId: string
  currentUserId: string
}

export default function PolicyTemplatesPage({ projectId, currentUserId }: Props) {
  const [templates, setTemplates] = useState<PolicyTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [role, setRole] = useState('viewer')
  const [open, setOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  const [versions, setVersions] = useState<PolicyTemplateVersion[]>([])
  const [historyName, setHistoryName] = useState('')
  const [mode, setMode] = useState<Mode>('create')
  const [selected, setSelected] = useState<PolicyTemplate | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [changeNote, setChangeNote] = useState('')
  const [params, setParams] = useState<PolicyParameters>(defaultPolicy())

  const canManage = role === 'owner' || role === 'admin'

  async function load() {
    setLoading(true)
    setError('')
    try {
      const [tplRes, membersRes] = await Promise.all([
        api.policies.listTemplates(projectId),
        api.projects.getMembers(projectId),
      ])
      setTemplates(tplRes.templates ?? [])
      // server ListMembers returns a bare array; tolerate the {members} wrapper too
      const memberList: ProjectMembership[] = Array.isArray(membersRes)
        ? membersRes
        : (membersRes.members ?? [])
      const me = memberList.find((m: ProjectMembership) => m.user_id === currentUserId)
      setRole(me?.role ?? 'viewer')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load policies')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [projectId])

  function openCreate() {
    setMode('create')
    setSelected(null)
    setName('')
    setDescription('')
    setChangeNote('')
    setParams(defaultPolicy())
    setOpen(true)
  }

  function openEdit(template: PolicyTemplate) {
    setMode('edit')
    setSelected(template)
    setName(template.name)
    setDescription(template.description)
    setChangeNote('')
    setParams(template.parameters)
    setOpen(true)
  }

  function openClone(template: PolicyTemplate) {
    setMode('clone')
    setSelected(template)
    setName(`${template.name} Copy`)
    setDescription(template.description)
    setChangeNote('')
    setParams(template.parameters)
    setOpen(true)
  }

  async function openHistory(template: PolicyTemplate) {
    setHistoryName(template.name)
    setVersions([])
    setHistoryOpen(true)
    try {
      const res = await api.policies.listVersions(template.id)
      setVersions(res.versions ?? [])
    } catch {
      setVersions([])
    }
  }

  async function submitTemplate(e: React.FormEvent) {
    e.preventDefault()
    if (!canManage) return
    try {
      if (mode === 'create') {
        await api.policies.createTemplate(projectId, { name, description, parameters: params })
      } else if (mode === 'clone' && selected) {
        await api.policies.cloneTemplate(selected.id, name)
      } else if (mode === 'edit' && selected) {
        await api.policies.updateTemplate(selected.id, { parameters: params, change_note: changeNote })
      }
      setOpen(false)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save template')
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Policy templates</h2>
          <p className="text-sm text-muted-foreground">Manage reusable access policy templates for this project.</p>
        </div>
        <Button onClick={openCreate} disabled={!canManage}><Plus className="w-4 h-4 mr-2" />New template</Button>
      </div>

      {!canManage && (
        <Card className="border-amber-200 bg-amber-50/60 dark:bg-amber-900/10">
          <CardContent className="pt-4 text-sm text-amber-800 dark:text-amber-300">
            You have read-only access. Only owner/admin can create, clone, or edit templates.
          </CardContent>
        </Card>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading templates…</p>
      ) : templates.length === 0 ? (
        <Card><CardContent className="pt-6 text-sm text-muted-foreground">No templates found.</CardContent></Card>
      ) : (
        <div className="space-y-3">
          {templates.map((template) => (
            <Card key={template.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-base">{template.name}</CardTitle>
                  <div className="flex items-center gap-2">
                    {template.is_system && <Badge variant="secondary">system</Badge>}
                    <Badge variant="outline">v{template.current_version}</Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                {template.description && <p className="text-sm text-muted-foreground mb-3">{template.description}</p>}
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => openHistory(template)}><History className="w-4 h-4 mr-1" />History</Button>
                  <Button variant="outline" size="sm" disabled={!canManage} onClick={() => openClone(template)}><Copy className="w-4 h-4 mr-1" />Clone</Button>
                  <Button size="sm" disabled={!canManage || template.is_system} onClick={() => openEdit(template)}>Edit</Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <PolicyTemplateEditorDialog
        open={open}
        mode={mode}
        name={name}
        description={description}
        changeNote={changeNote}
        params={params}
        onOpenChange={setOpen}
        onNameChange={setName}
        onDescriptionChange={setDescription}
        onChangeNoteChange={setChangeNote}
        onParamsChange={setParams}
        onSubmit={submitTemplate}
      />

      <PolicyTemplateHistoryDialog
        open={historyOpen}
        name={historyName}
        versions={versions}
        onOpenChange={setHistoryOpen}
      />
    </div>
  )
}
