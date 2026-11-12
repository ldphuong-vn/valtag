'use client'

import { useState, useEffect } from 'react'
import { api } from '@/lib/api-client'
import { CREDENTIAL_TYPES } from '@/lib/constants'
import type { Secret } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'

interface Props {
  open: boolean
  secret?: Secret | null
  projectId?: string | null
  onClose: () => void
  onSuccess: () => void
  onCreated?: (secretId: string) => void
}

export default function SecretForm({ open, secret, projectId, onClose, onSuccess, onCreated }: Props) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  // AX: default to the most common type — an empty select forced every user
  // through a required-field error on submit (dogfood 2026-11-12).
  const [credentialType, setCredentialType] = useState('api_key')
  const [source, setSource] = useState('')
  const [value, setValue] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (secret) {
      setName(secret.name)
      setDescription(secret.description ?? '')
      setCredentialType(secret.credential_type)
      setSource(secret.source ?? '')
      setValue('')
    } else {
      setName(''); setDescription(''); setCredentialType('api_key'); setSource(''); setValue('')
    }
    setError('')
  }, [secret, open])

  const isDirty = secret 
    ? (name !== secret.name || description !== (secret.description ?? '') || credentialType !== secret.credential_type || source !== (secret.source ?? ''))
    : (name !== '' || description !== '' || credentialType !== '' || source !== '' || value !== '')

  const handleClose = () => {
    if (isDirty) {
      if (window.confirm('You have unsaved changes. Are you sure you want to close?')) {
        onClose()
      }
    } else {
      onClose()
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!credentialType) {
      setError('Type is required')
      return
    }
    setLoading(true)
    setError('')
    try {
      if (secret) {
        await api.secrets.update(secret.id, { name, description, credential_type: credentialType, source })
        onSuccess()
      } else {
        const created = await api.secrets.create({
          name,
          description,
          credential_type: credentialType,
          source,
          value,
          project_id: projectId ?? undefined,
        })
        onCreated?.(created.id)
        onSuccess()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{secret ? 'Edit Secret' : 'Create Secret'}</DialogTitle>
          <DialogDescription>
            {secret ? 'Modify the details of your secret.' : 'Add a new secret to your vault.'}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="name">Name <span className="text-destructive">*</span></Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label>Type <span className="text-destructive">*</span></Label>
            <Select value={credentialType} onValueChange={setCredentialType}>
              <SelectTrigger><SelectValue placeholder="Select type" /></SelectTrigger>
              <SelectContent>
                {CREDENTIAL_TYPES.map((t) => (
                  <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          
          {!secret && (
            <div className="space-y-1.5">
              <Label htmlFor="value">Value <span className="text-destructive">*</span></Label>
              <Textarea id="value" value={value} onChange={(e) => setValue(e.target.value)} required rows={3} />
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="desc">Description</Label>
            <Input id="desc" value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          
          <div className="space-y-1.5">
            <Label htmlFor="source">Source</Label>
            <Input id="source" value={source} onChange={(e) => setSource(e.target.value)} />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={handleClose}>Cancel</Button>
            <Button type="submit" disabled={loading}>{secret ? 'Save changes' : 'Create secret'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
