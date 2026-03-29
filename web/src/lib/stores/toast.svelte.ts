let toast = $state<{ msg: string; type: 'success' | 'error' } | null>(null)
let timer: ReturnType<typeof setTimeout>

export const toastStore = {
  get current() { return toast },

  show(msg: string, type: 'success' | 'error' = 'success') {
    clearTimeout(timer)
    toast = { msg, type }
    timer = setTimeout(() => { toast = null }, 3000)
  },

  dismiss() {
    clearTimeout(timer)
    toast = null
  },
}
