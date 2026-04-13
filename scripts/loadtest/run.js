#!/usr/bin/env node
'use strict'

const { parseArgs, helpText, DEFAULTS, CITIES } = require('./config')
const { Metrics } = require('./metrics')
const { VirtualUser } = require('./user')

async function main() {
  const opts = parseArgs(process.argv.slice(2))
  if (opts.help) {
    process.stdout.write(helpText())
    return
  }

  if (typeof fetch !== 'function') {
    console.error(
      '[loadtest] Node 18+ is required (global fetch not available).',
    )
    process.exit(1)
  }

  console.log(
    `[loadtest] starting: server=${opts.server} users=${opts.users} ` +
      `duration=${opts.durationSec}s ramp=${opts.rampMs}ms ` +
      `interval=${opts.minIntervalMs}-${opts.maxIntervalMs}ms`,
  )

  const metrics = new Metrics()
  const users = []
  let stopping = false

  const reportTimer = setInterval(() => {
    console.log(metrics.progressLine(opts.users))
  }, DEFAULTS.reportIntervalMs)

  const shutdown = (reason) => {
    if (stopping) return
    stopping = true
    clearInterval(reportTimer)
    console.log(`[loadtest] shutting down (${reason})...`)
    for (const u of users) u.stop()
    // Give sockets a beat to flush disconnect frames before final report.
    setTimeout(() => {
      console.log(metrics.finalReport(opts.users))
      process.exit(0)
    }, 500)
  }

  process.on('SIGINT', () => shutdown('SIGINT'))
  process.on('SIGTERM', () => shutdown('SIGTERM'))

  setTimeout(() => shutdown('duration elapsed'), opts.durationSec * 1000)

  for (let i = 0; i < opts.users; i++) {
    if (stopping) break
    const u = new VirtualUser({
      id: i,
      serverUrl: opts.server,
      metrics,
      minIntervalMs: opts.minIntervalMs,
      maxIntervalMs: opts.maxIntervalMs,
    })
    users.push(u)
    u.start().catch((err) => {
      metrics.connectErrors++
      metrics.recordError(`start: ${err.message}`)
    })
    if (opts.rampMs > 0) {
      await new Promise((r) => setTimeout(r, opts.rampMs))
    }
  }

  const perCity = Math.ceil(users.length / CITIES.length)
  console.log(
    `[loadtest] ramp complete: ${users.length} users spawned ` +
      `across ${CITIES.length} cities (~${perCity}/city)`,
  )
}

main().catch((err) => {
  console.error('[loadtest] fatal', err)
  process.exit(1)
})
