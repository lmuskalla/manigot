// Toasts — action results and errors surface for a beat, then leave.

export interface Toast {
  id: number
  kind: 'ok' | 'error' | 'info'
  text: string
}

class ToastStore {
  items = $state<Toast[]>([])
  #next = 1

  push(kind: Toast['kind'], text: string, ttl = 4200) {
    const id = this.#next++
    this.items = [...this.items, { id, kind, text }]
    if (typeof setTimeout === 'function') {
      setTimeout(() => this.dismiss(id), ttl)
    }
  }

  ok(text: string) {
    this.push('ok', text)
  }

  error(text: string) {
    this.push('error', text, 7000)
  }

  info(text: string) {
    this.push('info', text)
  }

  dismiss(id: number) {
    this.items = this.items.filter((t) => t.id !== id)
  }
}

export const toasts = new ToastStore()
