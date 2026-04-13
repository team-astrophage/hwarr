'use strict'

class Metrics {
  constructor() {
    this.startedAt = Date.now()
    this.connected = 0
    this.connectErrors = 0
    this.disconnected = 0
    this.firesSent = 0
    this.firesOk = 0
    this.firesErr = 0
    this.viewportSubs = 0
    this.viewportErr = 0
    this.recv = { ignite: 0, update: 0, spread: 0, usersCount: 0 }
    this.errorSamples = new Map()
    this._lastFiresOk = 0
    this._lastTickAt = this.startedAt
  }

  recordError(message) {
    const key = (message || 'unknown').slice(0, 120)
    this.errorSamples.set(key, (this.errorSamples.get(key) || 0) + 1)
  }

  snapshot(totalUsers) {
    const now = Date.now()
    const elapsedMs = now - this.startedAt
    const deltaMs = now - this._lastTickAt
    const deltaOk = this.firesOk - this._lastFiresOk
    const rpsWindow = deltaMs > 0 ? (deltaOk / deltaMs) * 1000 : 0
    this._lastTickAt = now
    this._lastFiresOk = this.firesOk
    return {
      elapsedSec: Math.round(elapsedMs / 1000),
      totalUsers,
      connected: this.connected,
      connectErrors: this.connectErrors,
      firesSent: this.firesSent,
      firesOk: this.firesOk,
      firesErr: this.firesErr,
      rpsWindow: rpsWindow.toFixed(1),
      avgRps:
        elapsedMs > 0
          ? ((this.firesOk / elapsedMs) * 1000).toFixed(1)
          : '0.0',
      recv: { ...this.recv },
    }
  }

  progressLine(totalUsers) {
    const s = this.snapshot(totalUsers)
    return (
      `[loadtest] t=${String(s.elapsedSec).padStart(3, '0')}s ` +
      `users=${s.connected}/${s.totalUsers} ` +
      `fires sent=${s.firesSent} ok=${s.firesOk} err=${s.firesErr} ` +
      `rps=${s.rpsWindow} ` +
      `recv={ignite:${s.recv.ignite}, update:${s.recv.update}, spread:${s.recv.spread}}`
    )
  }

  finalReport(totalUsers) {
    const s = this.snapshot(totalUsers)
    const errs = [...this.errorSamples.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([msg, count]) => `${count}x "${msg}"`)
      .join(', ')
    return (
      `[loadtest] DONE duration=${s.elapsedSec}s ` +
      `users=${s.connected}/${totalUsers} ` +
      `fires ok=${s.firesOk} err=${s.firesErr} ` +
      `avg_rps=${s.avgRps} ` +
      `recv={ignite:${s.recv.ignite}, update:${s.recv.update}, spread:${s.recv.spread}, users:${s.recv.usersCount}}` +
      (errs ? `\n[loadtest] top errors: ${errs}` : '')
    )
  }
}

module.exports = { Metrics }
