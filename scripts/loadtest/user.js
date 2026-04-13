'use strict'

const { io } = require('socket.io-client')
const {
  randInt,
  cityForUser,
  fireNearCity,
  viewportForCity,
} = require('./config')

const HEARTBEAT_MS = 10_000

async function fetchToken(serverUrl) {
  const res = await fetch(`${serverUrl}/api/token`)
  if (!res.ok) throw new Error(`token fetch ${res.status}`)
  return res.json()
}

class VirtualUser {
  constructor({ id, serverUrl, metrics, minIntervalMs, maxIntervalMs }) {
    this.id = id
    this.serverUrl = serverUrl
    this.metrics = metrics
    this.minIntervalMs = minIntervalMs
    this.maxIntervalMs = maxIntervalMs
    this.city = cityForUser(id)
    // Draw exactly one fixed fire position at construction and keep firing
    // the same cell forever — models a user holding down the button on one
    // spot rather than spraying fires across the neighborhood.
    this.firePos = fireNearCity(this.city)
    this.socket = null
    this.heartbeatTimer = null
    this.fireTimer = null
    this.stopped = false
  }

  async start() {
    let auth
    try {
      auth = await fetchToken(this.serverUrl)
    } catch (err) {
      this.metrics.connectErrors++
      this.metrics.recordError(`token: ${err.message}`)
      return
    }

    // Match real browser behavior (client/src/lib/socketManager.ts) so a
    // transient Engine.IO ping timeout during ramp-up doesn't permanently
    // drop the virtual user.
    const socket = io(this.serverUrl, {
      transports: ['polling', 'websocket'],
      reconnection: true,
      reconnectionAttempts: 8,
      reconnectionDelay: 500,
      reconnectionDelayMax: 5_000,
      forceNew: true,
      auth: { user_id: auth.user_id, token: auth.token },
    })
    this.socket = socket

    socket.on('connect', () => {
      this.metrics.connected++
      // Clear any leftover timers from a previous lifecycle before spinning
      // up new ones — avoids duplicate heartbeat intervals on reconnect.
      this._clearTimers()
      socket.emit('heartbeat', { ts: Date.now() })
      this.heartbeatTimer = setInterval(() => {
        if (socket.connected) socket.emit('heartbeat', { ts: Date.now() })
      }, HEARTBEAT_MS)
      this._subscribeViewport()
      this._scheduleNextFire()
    })

    socket.on('connect_error', (err) => {
      this.metrics.connectErrors++
      this.metrics.recordError(`connect_error: ${err.message}`)
    })

    socket.on('disconnect', (reason) => {
      // connected is a live gauge: decrement on drop, re-increment on
      // reconnect's 'connect' event above.
      this.metrics.connected = Math.max(0, this.metrics.connected - 1)
      this.metrics.disconnected++
      if (reason && reason !== 'io client disconnect') {
        this.metrics.recordError(`disconnect: ${reason}`)
      }
      this._clearTimers()
    })

    socket.on('fire:ignite', () => {
      this.metrics.recv.ignite++
    })
    socket.on('fire:update', () => {
      this.metrics.recv.update++
    })
    socket.on('fire:spread', () => {
      this.metrics.recv.spread++
    })
    socket.on('users:count', () => {
      this.metrics.recv.usersCount++
    })
  }

  _subscribeViewport() {
    if (!this.socket || !this.socket.connected) return
    const vp = viewportForCity(this.city)
    this.socket.emit('subscribe:viewport', vp, (ack) => {
      if (ack && ack.status === 'ok') {
        this.metrics.viewportSubs++
      } else {
        this.metrics.viewportErr++
        this.metrics.recordError(
          `viewport: ${ack && ack.error ? ack.error : 'unknown'}`,
        )
      }
    })
  }

  _scheduleNextFire() {
    if (this.stopped) return
    const delay = randInt(this.minIntervalMs, this.maxIntervalMs)
    this.fireTimer = setTimeout(() => this._emitFire(), delay)
  }

  _emitFire() {
    if (this.stopped || !this.socket || !this.socket.connected) return
    const { lat, lng } = this.firePos
    this.metrics.firesSent++
    this.socket.emit('fire:ignite', { lat, lng }, (ack) => {
      if (ack && ack.status === 'ok') {
        this.metrics.firesOk++
      } else {
        this.metrics.firesErr++
        const msg = ack && ack.error ? ack.error : 'no ack'
        this.metrics.recordError(`fire:ignite: ${msg}`)
      }
    })
    this._scheduleNextFire()
  }

  _clearTimers() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
    if (this.fireTimer) {
      clearTimeout(this.fireTimer)
      this.fireTimer = null
    }
  }

  stop() {
    this.stopped = true
    this._clearTimers()
    if (this.socket) {
      this.socket.removeAllListeners()
      this.socket.disconnect()
      this.socket = null
    }
  }
}

module.exports = { VirtualUser }
