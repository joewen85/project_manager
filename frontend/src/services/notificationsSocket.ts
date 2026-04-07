type NotificationSocketListener = () => void

interface NotificationSocketState {
  socket: WebSocket | null
  socketURL: string
  listeners: Set<NotificationSocketListener>
  reconnectTimer: number | null
  reconnectDelayMs: number
  closeTimer: number | null
}

const stateKey = '__pm_notifications_socket_state__'
const isDebugEnabled = Boolean(import.meta.env.DEV)
const closeGracePeriodMs = 1200

const logSocket = (message: string, payload?: Record<string, unknown>) => {
  if (!isDebugEnabled) return
  if (payload) {
    console.debug(`[notifications-ws] ${message}`, payload)
    return
  }
  console.debug(`[notifications-ws] ${message}`)
}

const getState = (): NotificationSocketState => {
  const scope = window as typeof window & { [stateKey]?: NotificationSocketState }
  if (!scope[stateKey]) {
    scope[stateKey] = {
      socket: null,
      socketURL: '',
      listeners: new Set<NotificationSocketListener>(),
      reconnectTimer: null,
      reconnectDelayMs: 1000,
      closeTimer: null
    }
  }
  return scope[stateKey] as NotificationSocketState
}

const clearReconnectTimer = (state: NotificationSocketState) => {
  if (state.reconnectTimer !== null) {
    window.clearTimeout(state.reconnectTimer)
    state.reconnectTimer = null
  }
}

const clearCloseTimer = (state: NotificationSocketState) => {
  if (state.closeTimer !== null) {
    window.clearTimeout(state.closeTimer)
    state.closeTimer = null
  }
}

const emitMessage = (state: NotificationSocketState) => {
  state.listeners.forEach((listener) => {
    try {
      listener()
    } catch (error) {
      console.error(error)
    }
  })
}

const connect = (state: NotificationSocketState) => {
  if (!state.socketURL || state.listeners.size === 0) return
  if (state.socket && (state.socket.readyState === WebSocket.OPEN || state.socket.readyState === WebSocket.CONNECTING)) {
    return
  }

  const socket = new WebSocket(state.socketURL)
  state.socket = socket
  logSocket('connect:create', { url: state.socketURL, listeners: state.listeners.size })

  socket.onopen = () => {
    if (state.socket !== socket) {
      socket.close()
      return
    }
    state.reconnectDelayMs = 1000
    logSocket('socket:open', { listeners: state.listeners.size })
    emitMessage(state)
  }

  socket.onmessage = () => {
    logSocket('socket:message')
    emitMessage(state)
  }

  socket.onerror = () => {
    logSocket('socket:error', { readyState: socket.readyState })
    socket.close()
  }

  socket.onclose = (event) => {
    logSocket('socket:close', {
      code: event.code,
      reason: event.reason,
      wasClean: event.wasClean,
      isCurrent: state.socket === socket
    })
    if (state.socket !== socket) return
    state.socket = null
    if (state.listeners.size === 0) return

    clearReconnectTimer(state)
    const delay = state.reconnectDelayMs
    state.reconnectTimer = window.setTimeout(() => {
      state.reconnectTimer = null
      connect(state)
    }, delay)
    state.reconnectDelayMs = Math.min(state.reconnectDelayMs * 2, 30000)
    logSocket('reconnect:scheduled', { delay, nextDelay: state.reconnectDelayMs })
  }
}

export const subscribeNotificationSocket = (socketURL: string, listener: NotificationSocketListener) => {
  const state = getState()
  if (!socketURL) return () => {}

  clearCloseTimer(state)
  state.listeners.add(listener)

  if (state.socketURL && state.socketURL !== socketURL) {
    logSocket('socket:url-changed', { from: state.socketURL, to: socketURL })
    clearReconnectTimer(state)
    if (state.socket) {
      state.socket.close()
      state.socket = null
    }
  }

  state.socketURL = socketURL
  logSocket('subscribe', { listeners: state.listeners.size })
  connect(state)

  return () => {
    const currentState = getState()
    currentState.listeners.delete(listener)
    logSocket('unsubscribe', { listeners: currentState.listeners.size })
    if (currentState.listeners.size > 0) return

    clearReconnectTimer(currentState)
    clearCloseTimer(currentState)
    currentState.closeTimer = window.setTimeout(() => {
      currentState.closeTimer = null
      if (currentState.listeners.size > 0) return
      if (currentState.socket) {
        logSocket('socket:close-idle')
        currentState.socket.close()
        currentState.socket = null
      }
      currentState.socketURL = ''
      currentState.reconnectDelayMs = 1000
    }, closeGracePeriodMs)
  }
}
