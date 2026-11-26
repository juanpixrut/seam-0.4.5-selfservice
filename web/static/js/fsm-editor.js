/**
 * Seam FSM Editor — Lit shell
 *
 * The FSM engine (seam-fsm-editor.js) is loaded as a plain <script>
 * by the page and exposes a single global, SEAM_EDITOR. This module
 * provides the toolbar, theming, demo presets, and resize/layout
 * management.
 *
 * Engine API used (all members of the SEAM_EDITOR global):
 *   SEAM_EDITOR.initFSM(canvas)          — start the engine on a canvas element
 *   SEAM_EDITOR.draw()                   — redraw
 *   SEAM_EDITOR.zoomToFit()              — fit all nodes into the viewport
 *   SEAM_EDITOR.clearCanvas()            — reset nodes, links, viewport
 *   SEAM_EDITOR.restoreFromBackupData(d) — load a backup JSON object
 *   SEAM_EDITOR.getBackupData()          — export a backup JSON object
 *   SEAM_EDITOR.viewport                 — {x, y, k} — read for grid sync
 */

import { LitElement, html } from 'lit';

// ── Palette ───────────────────────────────────────────────────────────────────

function fsmPalette() {
  const dark = document.documentElement.classList.contains('dark');
  return {
    fg:       dark ? '#e2e8f0' : '#0f172a',
    selected: dark ? '#818cf8' : '#4f46e5',
    canvas:   dark ? '#1e293b' : '#f8fafc',
    gridLine: dark ? 'rgba(99,102,241,0.10)' : 'rgba(99,102,241,0.12)',
  };
}

// ── Canvas context colour proxy ───────────────────────────────────────────────

function themedContext(rawCtx) {
  return new Proxy(rawCtx, {
    set(target, prop, value) {
      if ((prop === 'strokeStyle' || prop === 'fillStyle') && typeof value === 'string') {
        const p = fsmPalette();
        if (value === 'black') value = p.fg;
        else if (value === 'blue') value = p.selected;
      }
      target[prop] = value;
      return true;
    },
    get(target, prop) {
      const v = target[prop];
      return typeof v === 'function' ? v.bind(target) : v;
    },
  });
}

// ── Demo presets (native backup format) ───────────────────────────────

const PRESETS = {
  lock: {
    nodeRadius: 30,
    nodes: [
      { x:  90, y: 200, text: 'Locked',   isAcceptState: false, textOnly: false },
      { x: 310, y: 200, text: 'Unlocked', isAcceptState: true,  textOnly: false },
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'unlock / key_valid', parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 0, text: 'lock',               parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
    ],
  },
  lifecycle: {
    nodeRadius: 30,
    nodes: [
      { x:  80, y: 200, text: 'New',          isAcceptState: false, textOnly: false },
      { x: 260, y: 100, text: 'Commissioned', isAcceptState: false, textOnly: false },
      { x: 440, y: 200, text: 'In Service',   isAcceptState: false, textOnly: false },
      { x: 260, y: 300, text: 'Retired',      isAcceptState: true,  textOnly: false },
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'commission',   parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'activate',     parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 1, text: 'maintain',     parallelPart: 0.5, perpendicularPart: -50, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 3, text: 'decommission', parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
    ],
  },
  extinguisher: {
    nodeRadius: 30,
    nodes: [
      { x:  80, y: 200, text: 'Due',         isAcceptState: false, textOnly: false },
      { x: 250, y: 200, text: 'Inspecting',  isAcceptState: false, textOnly: false },
      { x: 430, y: 100, text: 'Compliant',   isAcceptState: true,  textOnly: false },
      { x: 430, y: 300, text: 'Failed',      isAcceptState: false, textOnly: false },
      { x: 250, y: 340, text: 'Remediation', isAcceptState: false, textOnly: false },
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'begin',     parallelPart: 0.5, perpendicularPart: 0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'pass',      parallelPart: 0.5, perpendicularPart: 0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 3, text: 'fail',      parallelPart: 0.5, perpendicularPart: 0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 4, text: 'remediate', parallelPart: 0.5, perpendicularPart: 0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 0, text: 'resubmit',  parallelPart: 0.5, perpendicularPart: 0, lineAngleAdjust: 0 },
    ],
  },
  coldchain: {
    nodeRadius: 30,
    nodes: [
      { x: 200, y: 120, text: 'Nominal',      isAcceptState: false, textOnly: false },
      { x: 400, y:  80, text: 'Warning',      isAcceptState: false, textOnly: false },
      { x: 520, y: 240, text: 'Critical',     isAcceptState: false, textOnly: false },
      { x: 370, y: 330, text: 'Alert Sent',   isAcceptState: false, textOnly: false },
      { x: 150, y: 280, text: 'Acknowledged', isAcceptState: true,  textOnly: false },
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'temp_high',     parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 0, text: 'recovered',     parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'temp_critical', parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 3, text: 'auto_alert',    parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 4, text: 'ack',           parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 0, text: 'resolved',      parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
    ],
  },
  workorder: {
    nodeRadius: 30,
    nodes: [
      { x:  80, y: 220, text: 'Open',        isAcceptState: false, textOnly: false },
      { x: 250, y: 120, text: 'Assigned',    isAcceptState: false, textOnly: false },
      { x: 430, y: 120, text: 'In Progress', isAcceptState: false, textOnly: false },
      { x: 600, y: 220, text: 'Review',      isAcceptState: false, textOnly: false },
      { x: 430, y: 340, text: 'On Hold',     isAcceptState: false, textOnly: false },
      { x: 600, y: 340, text: 'Closed',      isAcceptState: true,  textOnly: false },
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'assign',   parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'start',    parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 3, text: 'complete', parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 5, text: 'approve',  parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 2, text: 'reject',   parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 4, text: 'hold',     parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 2, text: 'resume',   parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'SelfLink', node: 2, anchorAngle: -1.5707963, text: 'update' },
    ],
  },

  // ── Electronics demos ──────────────────────────────────────────────────────

  bms: {
    // Battery Management System: 8-state FSM covering charge, discharge,
    // cell balancing, over-temperature protection, and fault latch.
    nodeRadius: 30,
    nodes: [
      { x:  80, y: 280, text: 'Power Off',     isAcceptState: false, textOnly: false }, // 0
      { x: 240, y: 280, text: 'Self-Test',     isAcceptState: false, textOnly: false }, // 1
      { x: 420, y: 280, text: 'Standby',       isAcceptState: false, textOnly: false }, // 2
      { x: 420, y: 120, text: 'Charging',      isAcceptState: false, textOnly: false }, // 3
      { x: 420, y: 440, text: 'Discharging',   isAcceptState: false, textOnly: false }, // 4
      { x: 610, y: 120, text: 'Balancing',     isAcceptState: false, textOnly: false }, // 5
      { x: 630, y: 280, text: 'Over-Temp',     isAcceptState: false, textOnly: false }, // 6
      { x: 800, y: 280, text: 'Fault\nLatch',  isAcceptState: false, textOnly: false }, // 7
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'power_on',                          parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'self_test_pass',                    parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 7, text: 'self_test_fail',                    parallelPart: 0.5, perpendicularPart:  60, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 3, text: 'charger_on\n[V_bat < V_max]',       parallelPart: 0.5, perpendicularPart: -25, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 2, text: 'charge_done\n[SOC = 100%]',         parallelPart: 0.5, perpendicularPart: -25, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 4, text: 'load_on\n[V_bat > V_min]',          parallelPart: 0.5, perpendicularPart:  25, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 2, text: 'load_off',                          parallelPart: 0.5, perpendicularPart:  25, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 5, text: 'balance_req\n[ΔV > 20mV]',         parallelPart: 0.5, perpendicularPart: -25, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 5, nodeB: 3, text: 'balanced\n[ΔV < 5mV]',             parallelPart: 0.5, perpendicularPart: -25, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 6, text: 'temp_high\n[T > 45°C]',            parallelPart: 0.5, perpendicularPart:  35, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 6, text: 'temp_high\n[T > 45°C]',            parallelPart: 0.5, perpendicularPart: -35, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 6, nodeB: 2, text: 'temp_ok\n[T < 40°C]',              parallelPart: 0.5, perpendicularPart: -30, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 7, text: 'undervoltage\n[V_bat < 2.5V]',     parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 7, nodeB: 0, text: 'hw_reset',                          parallelPart: 0.5, perpendicularPart:  70, lineAngleAdjust: 0 },
      { type: 'SelfLink', node: 2, anchorAngle: -1.5707963, text: 'vtg_ok\n[wdog]' },
    ],
  },

  i2c: {
    // I2C Master Controller: 9-state FSM covering start condition, address
    // phase, ACK handling, data TX/RX, stop, arbitration loss, and NACK error.
    nodeRadius: 30,
    nodes: [
      { x:  80, y: 240, text: 'Bus\nIdle',      isAcceptState: true,  textOnly: false }, // 0
      { x: 240, y: 160, text: 'Start\nSent',    isAcceptState: false, textOnly: false }, // 1
      { x: 400, y: 160, text: 'Addr\nTX',       isAcceptState: false, textOnly: false }, // 2
      { x: 560, y: 160, text: 'Addr\nACK Wait', isAcceptState: false, textOnly: false }, // 3
      { x: 700, y:  80, text: 'Data\nWrite',    isAcceptState: false, textOnly: false }, // 4
      { x: 700, y: 240, text: 'Data\nRead',     isAcceptState: false, textOnly: false }, // 5
      { x: 560, y: 340, text: 'Stop\nSent',     isAcceptState: false, textOnly: false }, // 6
      { x: 240, y: 340, text: 'Arb\nLost',      isAcceptState: false, textOnly: false }, // 7
      { x: 400, y: 340, text: 'NACK\nError',    isAcceptState: false, textOnly: false }, // 8
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'send_start',                         parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'sda_stable\n[addr byte]',            parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 3, text: 'scl_9th_pulse',                      parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 4, text: 'ack_ok\n[write mode]',               parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 5, text: 'ack_ok\n[read mode]',                parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 3, text: 'byte_sent\n[more data]',             parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 6, text: 'byte_sent\n[last]',                  parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 5, nodeB: 6, text: 'byte_rcvd\n[last, send NACK]',       parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'SelfLink', node: 5, anchorAngle: 0, text: 'byte_rcvd\n[send ACK]' },
      { type: 'Link', nodeA: 6, nodeB: 0, text: 'bus_free',                           parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 7, text: 'arb_loss\n[SDA mismatch]',           parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 8, text: 'nack\n[no device]',                  parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 8, text: 'nack\n[write err]',                  parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 7, nodeB: 0, text: 'bus_released',                       parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 8, nodeB: 6, text: 'send_stop',                          parallelPart: 0.5, perpendicularPart: -30, lineAngleAdjust: 0 },
    ],
  },

  pwm_motor: {
    // Brushless DC Motor Controller: 9-state PWM drive FSM with ramp, speed
    // regulation, current limiting, stall detection, and fault latch.
    nodeRadius: 30,
    nodes: [
      { x:  80, y: 280, text: 'Fault\nReset',    isAcceptState: false, textOnly: false }, // 0
      { x:  80, y: 140, text: 'Initialise',       isAcceptState: false, textOnly: false }, // 1
      { x: 260, y: 140, text: 'Stopped',          isAcceptState: true,  textOnly: false }, // 2
      { x: 430, y:  80, text: 'Ramp\nUp',         isAcceptState: false, textOnly: false }, // 3
      { x: 600, y: 140, text: 'Running',          isAcceptState: false, textOnly: false }, // 4
      { x: 430, y: 200, text: 'Ramp\nDown',       isAcceptState: false, textOnly: false }, // 5
      { x: 600, y: 300, text: 'Current\nLimit',   isAcceptState: false, textOnly: false }, // 6
      { x: 430, y: 380, text: 'Stall\nDetect',    isAcceptState: false, textOnly: false }, // 7
      { x: 240, y: 380, text: 'Fault',            isAcceptState: false, textOnly: false }, // 8
    ],
    links: [
      { type: 'StartLink', node: 0, deltaX: -60, deltaY: 0, text: '' },
      { type: 'Link', nodeA: 0, nodeB: 1, text: 'wdog_ok\n[V_bus > 12V]',            parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 1, nodeB: 2, text: 'init_ok\n[drivers_rdy]',            parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 2, nodeB: 3, text: 'enable\n[setpoint > 0]',            parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 4, text: 'speed_ok\n[rpm ≈ target]',          parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 5, text: 'disable\n[setpoint = 0]',           parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 5, nodeB: 2, text: 'decel_done\n[rpm = 0]',             parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 6, text: 'i_high\n[I > I_lim]',              parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 6, nodeB: 4, text: 'i_ok\n[I < I_nom]',                parallelPart: 0.5, perpendicularPart: -40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 4, nodeB: 7, text: 'stall\n[rpm=0, I>0]',              parallelPart: 0.5, perpendicularPart:  40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 3, nodeB: 7, text: 'stall\n[no accel]',                parallelPart: 0.5, perpendicularPart:  40, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 7, nodeB: 8, text: 'timeout\n[t > 500ms]',             parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 6, nodeB: 8, text: 'i_critical\n[I > 2×I_lim]',        parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'Link', nodeA: 8, nodeB: 0, text: 'fault_reset\n[sw_reset]',           parallelPart: 0.5, perpendicularPart:   0, lineAngleAdjust: 0 },
      { type: 'SelfLink', node: 4, anchorAngle: 0, text: 'pid_tick\n[speed_adj]' },
    ],
  },
};

// ── Lit element ───────────────────────────────────────────────────────────────

class SeamFSMEditor extends LitElement {
  static properties = {
    fsmId:      { type: String, attribute: 'fsm-id' },
    fsmName:    { type: String, attribute: 'fsm-name' },
    csrfToken:  { type: String, attribute: 'csrf-token' },
    _status:    { state: true },
    _dark:      { state: true },
    _autoSize:  { state: true },
  };

  createRenderRoot() { return this; }

  firstUpdated() {
    const canvas = this.querySelector('#fsm-canvas');

    // Patch getContext: returns a colour-remapping proxy
    const origGetContext = canvas.getContext.bind(canvas);
    canvas.getContext = (type, ...args) => {
      const rawCtx = origGetContext(type, ...args);
      if (type === '2d') return themedContext(rawCtx);
      return rawCtx;
    };

    // Start the FSM engine
    SEAM_EDITOR.initFSM(canvas);

    // Wrap draw() so the grid stays in sync with pan/zoom after every redraw
    const origDraw = SEAM_EDITOR.draw;
    SEAM_EDITOR.draw = () => { origDraw(); this._syncGrid(canvas); };

    // Size canvas before loading data (zoomToFit needs canvas dimensions)
    this._resize(canvas);
    this._resizeObserver = new ResizeObserver(() => this._resize(canvas));
    this._resizeObserver.observe(this.querySelector('.fsm-canvas-wrap'));

    // Load saved data; zoomToFit after canvas is sized
    const dataEl = document.getElementById('fsm-data');
    if (dataEl) {
      try {
        const data = JSON.parse(dataEl.textContent);
        if (data && data.nodes) {
          SEAM_EDITOR.restoreFromBackupData(data);
          SEAM_EDITOR.zoomToFit();
        }
      } catch (_) {}
    }

    // Initial theme + grid
    this._dark = document.documentElement.classList.contains('dark');
    this._applyTheme(canvas);

    // Theme observer: set reactive state (triggers re-render) + redraw canvas
    this._themeObserver = new MutationObserver(() => {
      this._dark = document.documentElement.classList.contains('dark');
      this._applyTheme(canvas);
      if (typeof SEAM_EDITOR !== 'undefined') SEAM_EDITOR.draw();
    });
    this._themeObserver.observe(document.documentElement, {
      attributes: true, attributeFilter: ['class'],
    });
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._themeObserver?.disconnect();
    this._resizeObserver?.disconnect();
  }

  // Apply background colour, sync font global, update grid
  _applyTheme(canvas) {
    const dark = this._dark;
    const p = fsmPalette();
    canvas.style.backgroundColor = p.canvas;
    // Bridge: Lit theme state → SEAM_EDITOR theming properties
    if (typeof SEAM_EDITOR !== 'undefined') {
      SEAM_EDITOR.seamFont            = '12px system-ui, -apple-system, "Segoe UI", sans-serif';
      SEAM_EDITOR.seamNodeFill        = p.canvas;
      SEAM_EDITOR.seamAccentColor     = '#169B62';
      SEAM_EDITOR.seamNodeStroke      = 1.5;
      SEAM_EDITOR.seamLinkStroke      = 2.0;
      SEAM_EDITOR.seamNodeStrokeColor = dark ? '#818cf8' : '#6366f1';
      SEAM_EDITOR.seamNodeLabelColor  = dark ? '#e2e8f0' : '#1e293b';
      SEAM_EDITOR.seamLinkStrokeColor = dark ? '#94a3b8' : '#64748b';
      SEAM_EDITOR.seamLinkLabelColor  = dark ? '#94a3b8' : '#64748b';
      SEAM_EDITOR.seamEditBg          = dark ? 'rgba(30,41,59,0.97)'   : 'rgba(248,250,252,0.97)';
      SEAM_EDITOR.seamEditBorder      = dark ? 'rgba(129,140,248,0.7)' : 'rgba(99,102,241,0.5)';
    }
    this._syncGrid(canvas);
  }

  // Sync grid CSS to current viewport (pan + zoom)
  _syncGrid(canvas) {
    const p = fsmPalette();
    const size   = 24;
    const vp     = SEAM_EDITOR.viewport ?? { x: 0, y: 0, k: 1 };
    const scaled = size * vp.k;
    const ox     = vp.x % scaled;
    const oy     = vp.y % scaled;
    canvas.style.backgroundSize     = `${scaled}px ${scaled}px`;
    canvas.style.backgroundPosition = `${ox}px ${oy}px`;
    canvas.style.backgroundImage    = [
      `linear-gradient(to right,  ${p.gridLine} 1px, transparent 1px)`,
      `linear-gradient(to bottom, ${p.gridLine} 1px, transparent 1px)`,
    ].join(', ');
  }

  _resize(canvas) {
    const wrap = this.querySelector('.fsm-canvas-wrap');
    if (!wrap) return;
    const { width, height } = wrap.getBoundingClientRect();
    if (width > 0 && height > 0) {
      canvas.width  = width;
      canvas.height = height;
      if (typeof SEAM_EDITOR !== 'undefined') SEAM_EDITOR.draw();
    }
  }

  _loadPreset(e) {
    const key = e.target.value;
    if (!key || key === 'current') return;
    const preset = PRESETS[key];
    if (!preset) return;
    if (typeof SEAM_EDITOR !== 'undefined') {
      SEAM_EDITOR.clearCanvas();
      SEAM_EDITOR.restoreFromBackupData(preset);
      SEAM_EDITOR.zoomToFit();
    }
    e.target.value = 'current';
  }

  _exportJSON() {
    if (typeof SEAM_EDITOR === 'undefined') return;
    const data = SEAM_EDITOR.getBackupData();
    const el = document.createElement('a');
    el.href = 'data:application/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(data, null, 2));
    el.download = (this.fsmName || 'fsm') + '.json';
    document.body.appendChild(el); el.click(); document.body.removeChild(el);
  }

  _exportSVG() {
    if (typeof SEAM_EDITOR === 'undefined') return;
    const svg = SEAM_EDITOR.getSVGString();
    const blob = new Blob([svg], { type: 'image/svg+xml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = (this.fsmName || 'fsm') + '.svg';
    document.body.appendChild(a); a.click(); document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  _clearCanvas() {
    if (typeof SEAM_EDITOR === 'undefined') return;
    SEAM_EDITOR.clearCanvas();
    SEAM_EDITOR.draw();
  }

  // ── Zoom controls ────────────────────────────────────────────────────────
  // zoomBy() is implemented in seam-fsm-editor.js and zooms toward centre.
  _zoomIn()    { if (typeof SEAM_EDITOR !== 'undefined') SEAM_EDITOR.zoomBy(1.25); }
  _zoomOut()   { if (typeof SEAM_EDITOR !== 'undefined') SEAM_EDITOR.zoomBy(1 / 1.25); }
  _resetZoom() { if (typeof SEAM_EDITOR !== 'undefined') SEAM_EDITOR.zoomToFit(); }

  // ── Auto-size toggle ─────────────────────────────────────────────────────
  _toggleAutoSize() {
    this._autoSize = !this._autoSize;
    if (typeof SEAM_EDITOR !== 'undefined') {
      SEAM_EDITOR.seamAutoSize = this._autoSize;
      SEAM_EDITOR.draw();
    }
  }

  // ── PNG export via resvg ─────────────────────────────────────────────────
  _exportLaTeX(e) {
    if (typeof SEAM_EDITOR !== 'undefined') SEAM_EDITOR.saveAsLaTeX(e.shiftKey);
  }

  async _exportPNG() {
    if (typeof SEAM_EDITOR === 'undefined') return;
    const svg = SEAM_EDITOR.getSVGString();
    try {
      const res = await fetch('/fsm/export/png', {
        method: 'POST',
        headers: {
          'Content-Type': 'image/svg+xml',
          'X-CSRF-Token': this.csrfToken || '',
        },
        body: svg,
      });
      if (!res.ok) {
        const msg = await res.text();
        alert('PNG export failed: ' + msg);
        return;
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url; a.download = 'fsm-diagram.png'; a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      alert('PNG export error: ' + err.message);
    }
  }

  render() {
    const dark       = this._dark;
    const border     = dark ? '#334155' : '#e2e8f0';
    const toolbarBg  = dark ? '#1e293b' : '#ffffff';
    const hintCol    = dark ? '#64748b' : '#94a3b8';
    const textCol    = dark ? '#e2e8f0' : '#0f172a';
    const inputBorder= dark ? '#475569' : '#cbd5e1';
    const btnStyle   = [
      'display:inline-flex;align-items:center;gap:4px',
      'padding:5px 12px;font-size:13px;font-weight:500;border-radius:6px',
      `border:1px solid ${inputBorder}`,
      `background:${toolbarBg}`,
      `color:${textCol}`,
      'cursor:pointer',
    ].join(';');
    const selStyle   = [
      'font-size:13px;border-radius:6px',
      `border:1px solid ${inputBorder}`,
      `background:${toolbarBg}`,
      `color:${textCol}`,
      'padding:5px 8px;cursor:pointer',
    ].join(';');

    return html`
      <style>
        seam-fsm-editor { display:flex;flex-direction:column;height:100%;width:100%; }
        .fsm-toolbar {
          display:flex;align-items:center;gap:8px;
          padding:8px 12px;flex-shrink:0;flex-wrap:wrap;
          border-bottom:1px solid ${border};
          background:${toolbarBg};
        }
        .fsm-hint { flex:1;font-size:12px;color:${hintCol}; }
        .fsm-canvas-wrap { flex:1;overflow:hidden;position:relative; }
        #fsm-canvas { display:block;width:100%;height:100%;cursor:crosshair; }
      </style>

      <div class="fsm-toolbar">
        <span class="fsm-hint">
          Double-click: add state &nbsp;·&nbsp;
          Shift-click: toggle accept state &nbsp;·&nbsp;
          Drag background: pan &nbsp;·&nbsp;
          Scroll: zoom &nbsp;·&nbsp;
          Delete: remove selected
        </span>
        ${this._status ? html`
          <span style="font-size:13px;color:${dark ? '#818cf8' : '#6366f1'}">${this._status}</span>
        ` : ''}
        <select style="${selStyle}" @change=${this._loadPreset}>
          <option value="current">Load demo…</option>
          <option value="lock">1 · Lock / Unlock</option>
          <option value="lifecycle">2 · Asset Lifecycle</option>
          <option value="extinguisher">3 · Extinguisher Inspection</option>
          <option value="coldchain">4 · Cold Chain Monitor</option>
          <option value="workorder">5 · Work Order</option>
          <option disabled>──────────────</option>
          <option value="bms">6 · Battery Management System</option>
          <option value="i2c">7 · I2C Master Controller</option>
          <option value="pwm_motor">8 · PWM Motor Controller</option>
        </select>
        <button style="${btnStyle}" @click=${this._clearCanvas}>
          <span class="material-icons" style="font-size:16px">clear_all</span>Clear
        </button>
        <button style="${btnStyle}" @click=${this._zoomOut}>
          <span class="material-icons" style="font-size:16px">zoom_out</span>
        </button>
        <button style="${btnStyle}" @click=${this._resetZoom} title="Fit to screen">
          <span class="material-icons" style="font-size:16px">fit_screen</span>
        </button>
        <button style="${btnStyle}" @click=${this._zoomIn}>
          <span class="material-icons" style="font-size:16px">zoom_in</span>
        </button>
        <button
          style="${btnStyle}${this._autoSize ? `;background:${dark ? '#4338ca' : '#6366f1'};color:#fff;border-color:transparent` : ''}"
          @click=${this._toggleAutoSize}
          title="Adapt node size to label">
          <span class="material-icons" style="font-size:16px">format_size</span>
        </button>
        <button style="${btnStyle}" @click=${this._exportJSON}>
          <span class="material-icons" style="font-size:16px">download</span>JSON
        </button>
        <button style="${btnStyle}" @click=${this._exportSVG} title="Export as SVG">
          <span class="material-icons" style="font-size:16px">polyline</span>SVG
        </button>
        <button style="${btnStyle}" @click=${this._exportPNG} title="Export as PNG (requires resvg)">
          <span class="material-icons" style="font-size:16px">image</span>PNG
        </button>
        <button style="${btnStyle}" @click=${this._exportLaTeX} title="Export as LaTeX / TikZ (Shift for monochrome)">
          <span style="font-family:Georgia,serif;font-size:15px;font-weight:bold">L</span>
        </button>
      </div>

      <div class="fsm-canvas-wrap">
        <canvas id="fsm-canvas"></canvas>
      </div>
    `;
  }
}

customElements.define('seam-fsm-editor', SeamFSMEditor);
