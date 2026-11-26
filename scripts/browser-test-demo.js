// Browser test of the form engine's script-driven widgets (input mask, barcode, gps,
// signature, any-picker, asset-picker, icon-picker) in a repeatable's server-rendered row
// and in a row added with the Add button, in the light and the dark theme. It fails on any
// 4xx/5xx response (except map tiles) and any page error, and checks what is visible
// (computed colours, element sizes), not only DOM state.
//
//   SEAM_DEMO_DUMP=$PWD/build/demo.html go test ./internal/ui -run TestDemoKitchen_DumpPage
//     (writes one file per page of the Demo module: build/demo-inputs.html, build/demo-rows.html, ...)
//   PUPPETEER_CORE=<path to puppeteer-core> CHROME_BIN=<chromium> node scripts/browser-test-demo.js build/demo.html
//
// STATIC_ROOT overrides web/static (to run against a modified copy of app.js or app.css).
// Dormant guard: listed in docs/KNOWN_ISSUES.md.
if (!process.env.CHROME_BIN) { console.error('set CHROME_BIN to a Chromium executable'); process.exit(2); }
const puppeteer = require(process.env.PUPPETEER_CORE || 'puppeteer-core');
const http = require('http'), fs = require('fs'), path = require('path');
const ROOT = process.env.STATIC_ROOT || path.join(__dirname, '..', 'web', 'static');
const PAGE = process.argv[2] || 'build/demo.html';
const MIME = { '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png', '.json': 'application/json', '.woff2': 'font/woff2', '.svg': 'image/svg+xml' };

const server = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://x');
  // /demo/<page>: the dump of that page, written next to PAGE (build/demo.html -> build/demo-<page>.html)
  const m = /^\/demo\/([a-z]+)$/.exec(u.pathname);
  if (m) {
    const f = PAGE.slice(0, PAGE.length - path.extname(PAGE).length) + '-' + m[1] + path.extname(PAGE);
    if (!fs.existsSync(f)) { res.statusCode = 404; return res.end('no dump of the page ' + m[1] + ': ' + f); }
    res.setHeader('content-type', 'text/html'); return res.end(fs.readFileSync(f));
  }
  if (u.pathname === '/picker/search') {
    res.setHeader('content-type', 'application/json');
    if (u.searchParams.get('id')) return res.end(JSON.stringify([{ id: +u.searchParams.get('id'), label: 'Pump ' + u.searchParams.get('id') }]));
    return res.end(JSON.stringify([{ id: 11, label: 'Pump A' }, { id: 12, label: 'Pump B' }]));
  }
  if (u.pathname.startsWith('/picker/icons')) { res.setHeader('content-type', 'application/json'); return res.end('[{"id":"home","label":"home"}]'); }
  for (const rel of [u.pathname.replace(/^\/static/, ''), u.pathname]) {
    const f = path.join(ROOT, rel);
    if (f.startsWith(ROOT) && fs.existsSync(f) && fs.statSync(f).isFile()) {
      res.setHeader('content-type', MIME[path.extname(f)] || 'application/octet-stream');
      return res.end(fs.readFileSync(f));
    }
  }
  res.statusCode = 404; res.end('nf');
});

const results = [];
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function runTheme(browser, base, theme) {
  const check = (name, ok, detail) => results.push([ok, `[${theme}] ${name}`, detail]);
  const page = await browser.newPage();
  await page.setViewport({ width: 1100, height: 1500 });
  const problems = [];
  // search.svg: the autocomplete vendor stylesheet names it and autocomplete-seam.css deliberately
  // does not vendor it (documented there); a known, harmless 404 outside this test's scope.
  page.on('response', (r) => { if (r.status() >= 400 && !/openstreetmap|vendor\/images\/search\.svg/.test(r.url())) problems.push(r.status() + ' ' + r.url().replace(base, '')); });
  page.on('pageerror', (e) => problems.push('pageerror: ' + e.message));
  // the demonstrations are on two pages of the module: Inputs, then Rows
  const open = async (group) => {
    await page.goto(base + '/demo/' + group, { waitUntil: 'load' });
    await sleep(600);
    if (theme === 'dark') { await page.evaluate(() => document.documentElement.classList.add('dark')); await sleep(200); }
  };
  await open('inputs');

  // a stored GPS value: opening the panel shows a real map, tiles and an SVG pin (no marker image)
  await page.click('button[data-tab="demo-tab-kitchen_field"]');
  await sleep(300);
  const depotRoot = await page.evaluateHandle(() => document.getElementById('gps-map-depot_location').closest('[data-seam-widget="gps"]'));
  await (await depotRoot.$('[data-gps-toggle]')).click();
  await sleep(800);
  const d = await page.evaluate((root) => { const m = root.querySelector('[data-gps-map]'); const pin = m.querySelector('.leaflet-marker-icon'); return { w: m.clientWidth, h: m.clientHeight, tiles: m.querySelectorAll('.leaflet-tile').length, svgPin: !!(pin && pin.querySelector('svg')), imgs: m.querySelectorAll('img.leaflet-marker-icon,img.leaflet-marker-shadow').length }; }, depotRoot);
  check('stored GPS value: panel opens with a sized map, tiles and an SVG pin', d.w > 0 && d.h > 0 && d.tiles >= 4 && d.svgPin && d.imgs === 0, JSON.stringify(d));
  await (await depotRoot.$('[data-gps-close]')).click(); // its open panel would cover the next field's button
  const yardRoot = await page.evaluateHandle(() => document.getElementById('gps-map-yard_location').closest('[data-seam-widget="gps"]'));
  await (await yardRoot.$('[data-gps-toggle]')).click();
  await sleep(800);
  const y = await page.evaluate((root) => { const p = root.querySelector('[data-gps-panel]'); return { wide: p.classList.contains('fe-gps-panel--wide'), pw: Math.round(p.getBoundingClientRect().width), rw: Math.round(root.getBoundingClientRect().width), tiles: root.querySelector('[data-gps-map]').querySelectorAll('.leaflet-tile').length }; }, yardRoot);
  check('x-seam-panel: wide opens at the full width of the field', y.wide && Math.abs(y.pw - y.rw) <= 2 && y.tiles >= 4, JSON.stringify(y));

  // asset-picker (Choices tab) lists assets as soon as it has focus, then stores the choice
  await page.click('button[data-tab="demo-tab-kitchen_choices"]');
  await sleep(300);
  const ap2 = '#demo-tab-kitchen_choices [data-seam-picker][data-entity="entity:assets"]';
  await page.focus(ap2 + ' input[type=text]');
  await page.waitForSelector(ap2 + ' li', { timeout: 4000 }).catch(() => {});
  const n2 = await page.$$eval(ap2 + ' li', (l) => l.length);
  if (n2) await page.click(ap2 + ' li');
  const v2 = await page.$eval(ap2 + ' input[type="hidden"]', (e) => e.value);
  check('asset-picker lists options on focus, without typing', n2 > 0 && v2 === '11', `items=${n2} id=${v2}`);

  await open('rows');
  await page.click('button[data-tab="demo-tab-kitchen_rows"]');
  await sleep(500);

  async function exercise(i, label) {
    const rowSel = `#demo-tab-kitchen_rows .rep-rows > .repeatable-row:nth-child(${i + 1})`;
    const q = (s) => `${rowSel} ${s}`;
    const exists = await page.$(rowSel);
    check(`${label}: row exists`, !!exists, rowSel);
    if (!exists) return;
    // masks
    await page.focus(q('input[data-seam-mask][name$="[tag_code]"]')); await page.keyboard.type('ab1234');
    const tag = await page.$eval(q('input[data-seam-mask][name$="[tag_code]"]'), (e) => e.value);
    check(`${label}: text mask AA-9999`, tag === 'AB-1234', tag);
    await page.focus(q('input[data-seam-mask][name$="[phone]"]')); await page.keyboard.type('5551234567');
    const ph = await page.$eval(q('input[data-seam-mask][name$="[phone]"]'), (e) => e.value);
    check(`${label}: tel mask 999-999-9999`, ph === '555-123-4567', ph);
    // a barcode field with no code yet takes no room (an empty <svg> would reserve the default 300x150)
    const emptyH = await page.$eval(q('[data-seam-widget="barcode"] svg'), (s) => Math.round(s.getBoundingClientRect().height));
    check(`${label}: an empty barcode field reserves no space`, emptyH === 0, String(emptyH));
    // barcode: drawn on white, updates as the code changes, wrong check digit turns the field red whatever the theme
    const bcIn = q('[data-seam-widget="barcode"] input[type="text"]');
    await page.focus(bcIn); await page.keyboard.type('5901234123457'); await sleep(250);
    const bc = await page.$eval(q('[data-seam-widget="barcode"]'), (w) => { const s = w.querySelector('svg'); const r = s.querySelector('rect'); const b = s.getBoundingClientRect(); return { nodes: s.querySelectorAll('rect,path,text').length, fill: r && getComputedStyle(r).fill, w: Math.round(b.width), h: Math.round(b.height), aria: w.querySelector('input').getAttribute('aria-invalid') }; });
    check(`${label}: barcode drawn, sized, on a white background`, bc.nodes > 0 && bc.w > 0 && bc.h > 0 && bc.fill === 'rgb(255, 255, 255)' && bc.aria === null, JSON.stringify(bc));
    await page.keyboard.press('Backspace'); await page.keyboard.type('8'); await sleep(250);
    const bad = await page.$eval(bcIn, (e) => ({ aria: e.getAttribute('aria-invalid'), border: getComputedStyle(e).borderColor }));
    const cleared = await page.$eval(q('[data-seam-widget="barcode"] svg'), (s) => s.querySelectorAll('rect,path').length);
    check(`${label}: wrong check digit: red border while focused, preview cleared`, bad.aria === 'true' && bad.border === 'rgb(239, 68, 68)' && cleared === 0, JSON.stringify({ ...bad, cleared }));
    await page.keyboard.press('Backspace'); await page.keyboard.type('7'); await sleep(250);
    const again = await page.$eval(q('[data-seam-widget="barcode"]'), (w) => ({ aria: w.querySelector('input').getAttribute('aria-invalid'), nodes: w.querySelector('svg').querySelectorAll('rect,path,text').length }));
    check(`${label}: correcting the code redraws the barcode and clears the flag`, again.aria === null && again.nodes > 0, JSON.stringify(again));
    // gps: a dropdown panel, map built when opened, SVG pin, Done closes
    const gps = q('[data-seam-widget="gps"]');
    const closed = await page.$eval(gps, (w) => ({ hidden: w.querySelector('[data-gps-panel]').hidden, built: w.querySelector('[data-gps-map]').classList.contains('leaflet-container') }));
    check(`${label}: gps panel starts closed, map not built yet`, closed.hidden && !closed.built, JSON.stringify(closed));
    await page.click(gps + ' [data-gps-toggle]'); await sleep(800);
    const opened = await page.$eval(gps, (w) => { const m = w.querySelector('[data-gps-map]'); return { hidden: w.querySelector('[data-gps-panel]').hidden, w: m.clientWidth, h: m.clientHeight, tiles: m.querySelectorAll('.leaflet-tile').length }; });
    check(`${label}: gps panel opens with a sized map and tiles`, !opened.hidden && opened.w > 0 && opened.h > 0 && opened.tiles >= 4, JSON.stringify(opened));
    const geo = await page.$eval(gps, (w) => { const p = w.querySelector('[data-gps-panel]'); const pr = p.getBoundingClientRect(), tr = w.querySelector('[data-gps-toggle]').getBoundingClientRect(); return { dRight: Math.round(pr.right - tr.right), left: Math.round(pr.left), below: pr.top >= tr.bottom - 1, leftClass: p.classList.contains('fe-gps-panel--left') }; });
    check(`${label}: gps panel sits under the button, right-aligned (left-anchored only when it would leave the screen)`, geo.below && geo.left >= 0 && (Math.abs(geo.dRight) <= 2 || geo.leftClass), JSON.stringify(geo));
    await page.click(gps + ' [data-gps-expand]'); await sleep(700);
    const wideM = await page.$eval(gps, (w) => { const p = w.querySelector('[data-gps-panel]'); const m = w.querySelector('[data-gps-map]'); return { pw: Math.round(p.getBoundingClientRect().width), rw: Math.round(w.getBoundingClientRect().width), mh: m.clientHeight, tiles: m.querySelectorAll('.leaflet-tile').length, pos: getComputedStyle(p).position, pressed: w.querySelector('[data-gps-expand]').getAttribute('aria-pressed'), rowW: w.closest('.repeatable-row') ? Math.round(w.closest('.repeatable-row').getBoundingClientRect().width) : null }; });
    check(`${label}: expand opens the panel at the full width of the field, in the flow, and of the whole row in a repeatable`, Math.abs(wideM.pw - wideM.rw) <= 2 && (wideM.rowW === null || wideM.rowW - wideM.pw <= 40) && wideM.pos === 'static' && wideM.mh >= 300 && wideM.tiles >= 4 && wideM.pressed === 'true', JSON.stringify(wideM));
    await page.click(gps + ' [data-gps-expand]'); await sleep(400);
    const backM = await page.$eval(gps, (w) => ({ pos: getComputedStyle(w.querySelector('[data-gps-panel]')).position, pressed: w.querySelector('[data-gps-expand]').getAttribute('aria-pressed') }));
    check(`${label}: expanding again returns to the dropdown`, backM.pos === 'absolute' && backM.pressed === 'false', JSON.stringify(backM));
    const box = await (await page.$(gps + ' [data-gps-map]')).boundingBox();
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2); await sleep(400);
    const placed = await page.$eval(gps, (w) => { const m = w.querySelector('[data-gps-map]'); const pin = m.querySelector('.leaflet-marker-icon'); const b = pin && pin.getBoundingClientRect(); return { value: w.querySelector('input[type=text]').value, svg: !!(pin && pin.querySelector('svg')), w: b && Math.round(b.width), imgs: m.querySelectorAll('img.leaflet-marker-icon,img.leaflet-marker-shadow').length }; });
    check(`${label}: clicking the map posts lat,lng and shows an SVG pin`, /^-?\d+\.\d{6},-?\d+\.\d{6}$/.test(placed.value) && placed.svg && placed.w > 0 && placed.imgs === 0, JSON.stringify(placed));
    await page.click(gps + ' [data-gps-close]');
    const shut = await page.$eval(gps, (w) => w.querySelector('[data-gps-panel]').hidden);
    check(`${label}: Done closes the gps panel`, shut === true, String(shut));
    // signature
    const cv = await (await page.$(q('canvas'))).boundingBox();
    await page.mouse.move(cv.x + 20, cv.y + 20); await page.mouse.down();
    await page.mouse.move(cv.x + 80, cv.y + 60, { steps: 5 }); await page.mouse.move(cv.x + 140, cv.y + 30, { steps: 5 });
    await page.mouse.up();
    const sig = await page.$eval(q('[data-seam-widget="signature"] input[type="hidden"]'), (e) => e.value.slice(0, 22));
    check(`${label}: signature serialised as a PNG data URL`, sig === 'data:image/png;base64,', sig);
    // any-picker with a Browse button, searching live and storing the chosen id
    const ap = q('[data-seam-picker][data-entity="entity:assets"]');
    const hasBrowse = await page.$eval(ap, (w) => !!w.querySelector('button[onclick*="SeamPicker.browse"]') && w.dataset.selector === 'assets');
    await page.focus(ap + ' input[type="text"]'); await page.keyboard.type('pu');
    await page.waitForSelector(ap + ' li', { timeout: 4000 }).catch(() => {});
    const items = await page.$$eval(ap + ' li', (l) => l.length);
    if (items) await page.click(ap + ' li');
    const val = await page.$eval(ap + ' input[type="hidden"]', (e) => e.value);
    check(`${label}: any-picker has Browse, searches and stores the chosen id`, hasBrowse && items > 0 && val === '11', `browse=${hasBrowse} items=${items} id=${val}`);
    const ip = await page.$eval(q('[data-seam-picker][data-preview="icon"]'), (e) => e.dataset.pkReady);
    check(`${label}: icon-picker initialised`, ip === '1', String(ip));
    // no icon chosen: the place for it is drawn as an empty slot of the icon's own size
    const slot = await page.$eval(q('[data-seam-picker][data-preview="icon"] [data-icon-preview]'), (e) => { const s = getComputedStyle(e), r = e.getBoundingClientRect(); return { text: e.textContent, style: s.borderTopStyle, bw: s.borderTopWidth, w: Math.round(r.width), h: Math.round(r.height), mark: getComputedStyle(e, '::before').content }; });
    check(`${label}: icon-picker without an icon shows an empty slot`, slot.text === '' && slot.style === 'dashed' && slot.bw !== '0px' && slot.w === 28 && slot.h === 28 && slot.mark.includes('more_horiz'), JSON.stringify(slot));
  }

  const rows0 = await page.$$eval('#demo-tab-kitchen_rows .rep-rows > .repeatable-row', (r) => r.length);
  check('server-rendered rows present', rows0 >= 1, String(rows0));
  await exercise(0, 'row 1 (server-rendered)').catch((e) => check('row 1 (server-rendered): aborted', false, e.message));
  await page.click('#demo-tab-kitchen_rows button[onclick*="SeamRepeatable.add"]');
  await sleep(500);
  const rows1 = await page.$$eval('#demo-tab-kitchen_rows .rep-rows > .repeatable-row', (r) => r.length);
  check('Add creates a row', rows1 === rows0 + 1, `${rows0} -> ${rows1}`);
  await exercise(rows0, 'added row').catch((e) => check('added row: aborted', false, e.message));

  await page.click(`#demo-tab-kitchen_rows .rep-rows > .repeatable-row:nth-child(${rows0 + 1}) [data-seam-widget="signature"] button`);
  const sigs = await page.$$eval('#demo-tab-kitchen_rows [data-seam-widget="signature"] input[type="hidden"]', (l) => l.map((e) => e.value.length > 0));
  check('Clear in the added row clears only that row', sigs[0] === true && sigs[rows0] === false, JSON.stringify(sigs));
  const names = await page.$$eval('#demo-tab-kitchen_rows .rep-rows input[name]', (l) => l.map((e) => e.name));
  check('field names are indexed per row', names.includes('site_visits[0][location]') && names.some((n) => /^site_visits\[[1-9]\d*\]\[location\]$/.test(n)), names.filter((n) => /location/.test(n)).join(','));
  check('no failed requests and no page errors', problems.length === 0, [...new Set(problems)].join(' | ').slice(0, 400));
  await page.close();
}

(async () => {
  await new Promise((r) => server.listen(0, r));
  const base = 'http://127.0.0.1:' + server.address().port;
  const browser = await puppeteer.launch({ executablePath: process.env.CHROME_BIN, args: ['--no-sandbox'], headless: true });
  for (const theme of ['light', 'dark']) await runTheme(browser, base, theme);
  await browser.close(); server.close();
  let failed = 0;
  for (const [ok, name, detail] of results) { if (!ok) failed++; console.log((ok ? 'PASS' : 'FAIL') + '  ' + name + (ok ? '' : '   <- ' + detail)); }
  console.log(`\n${results.length - failed}/${results.length} passed`);
  process.exit(failed ? 1 : 0);
})().catch((e) => { console.error('harness error:', e); process.exit(2); });
