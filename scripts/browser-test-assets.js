// Browser test of the Assets form, loaded the way users get it: an htmx fetch of the form
// fragment into a page, in the light and the dark theme. Checks the "Map & photos" tab (GPS
// dropdown, SVG pin, photo gallery), the compact GPS buttons, and the related-asset picker
// (stored label resolved, list on focus and when emptied, typed search, choice stored).
// Fails on any 4xx/5xx response (except map tiles) and any page error.
//
//   SEAM_ASSET_DUMP=$PWD/build/asset-form.html go test ./internal/ui -run TestAssetForm_DumpPage
//   PUPPETEER_CORE=<path to puppeteer-core> CHROME_BIN=<chromium> node scripts/browser-test-assets.js build/asset-form.html
//
// STATIC_ROOT overrides web/static. Dormant guard: listed in docs/KNOWN_ISSUES.md.
if (!process.env.CHROME_BIN) { console.error('set CHROME_BIN to a Chromium executable'); process.exit(2); }
const puppeteer = require(process.env.PUPPETEER_CORE || 'puppeteer-core');
const http = require('http'), fs = require('fs'), path = require('path');
const ROOT = process.env.STATIC_ROOT || path.join(__dirname, '..', 'web', 'static');
const FORM = process.argv[2] || 'build/asset-form.html';
const MIME = { '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png', '.json': 'application/json', '.woff2': 'font/woff2', '.svg': 'image/svg+xml' };
const SHELL = `<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/css/tailwind.css"><link rel="stylesheet" href="/css/app.css"><link rel="stylesheet" href="/vendor/material-icons.css"><script src="/vendor/htmx@1.9.10.min.js"></script><script src="/js/app.js" defer></script></head><body class="bg-gray-50 dark:bg-gray-900"><div id="host" class="p-6" style="max-width:56rem" hx-get="/assets/dynamic/7/edit" hx-trigger="load" hx-swap="innerHTML"></div></body></html>`;

const server = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://x');
  if (u.pathname === '/') { res.setHeader('content-type', 'text/html'); return res.end(SHELL); }
  if (u.pathname === '/assets/dynamic/7/edit') { res.setHeader('content-type', 'text/html'); return res.end(fs.readFileSync(FORM)); }
  if (u.pathname === '/picker/search') {
    res.setHeader('content-type', 'application/json');
    if (u.searchParams.get('id')) return res.end(JSON.stringify([{ id: +u.searchParams.get('id'), label: 'Pump ' + u.searchParams.get('id') }]));
    return res.end(JSON.stringify([{ id: 11, label: 'Pump A' }, { id: 12, label: 'Pump B' }]));
  }
  if (u.pathname === '/gallery/items') { res.setHeader('content-type', 'application/json'); return res.end('{"items":[]}'); }
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
  await page.setViewport({ width: 1100, height: 1400 });
  const problems = [];
  // search.svg: named by the autocomplete vendor stylesheet, deliberately not vendored (see
  // autocomplete-seam.css); a known, harmless 404.
  page.on('response', (r) => { if (r.status() >= 400 && !/openstreetmap|vendor\/images\/search\.svg/.test(r.url())) problems.push(r.status() + ' ' + r.url().replace(base, '')); });
  page.on('pageerror', (e) => problems.push('pageerror: ' + e.message));
  await page.goto(base + '/', { waitUntil: 'load' });
  await page.waitForSelector('#host [data-seam-widget="gps"]', { timeout: 6000 }).catch(() => {});
  await sleep(700);
  if (theme === 'dark') { await page.evaluate(() => document.documentElement.classList.add('dark')); await sleep(200); }
  const clickTab = (text) => page.evaluate((text) => {
    const b = [...document.querySelectorAll('button')].find((e) => e.textContent.trim() === text);
    if (b) b.click();
    return !!b;
  }, text);

  // the fragment was requested with ?tab=tab-geo: that tab is open on load, the first one is not
  const first = await page.evaluate(() => { const g = document.getElementById('tab-geo'), m = document.getElementById('tab-principal'); return { geo: !!g && getComputedStyle(g).display !== 'none', main: !!m && getComputedStyle(m).display === 'none' }; });
  check('?tab=tab-geo opens the Map & photos tab on load', first.geo && first.main, JSON.stringify(first));

  // related-asset picker (Location tab): stored label, list on focus / when emptied, search, choice
  check('Location tab found', await clickTab('Location'), '');
  await sleep(300);
  const pk = '[data-seam-picker][data-entity="entity:assets"]';
  await page.waitForFunction((pk) => { const i = document.querySelector(pk + ' input[type=text]'); return i && i.value === 'Pump 3'; }, { timeout: 3000 }, pk).catch(() => {});
  let st = await page.$eval(pk, (w) => ({ text: w.querySelector('input[type=text]').value, id: w.querySelector('input[type=hidden]').value, ready: w.dataset.pkReady }));
  check('related asset: stored id 3 resolves to its label', st.text === 'Pump 3' && st.id === '3' && st.ready === '1', JSON.stringify(st));
  await page.focus(pk + ' input[type=text]');
  await page.keyboard.down('Control'); await page.keyboard.press('KeyA'); await page.keyboard.up('Control');
  await page.keyboard.press('Backspace');
  await page.waitForSelector(pk + ' li', { timeout: 4000 }).catch(() => {});
  const listed = await page.$$eval(pk + ' li', (l) => l.length);
  const stale = await page.$eval(pk + ' input[type=hidden]', (e) => e.value);
  check('related asset: emptying the box lists assets and drops the stale id', listed >= 2 && stale === '', `items=${listed} hidden="${stale}"`);
  await page.keyboard.type('pu');
  await sleep(700);
  const typed = await page.$$eval(pk + ' li', (l) => l.length);
  check('related asset: typing searches', typed >= 1, String(typed));
  const li = await page.$(pk + ' li');
  if (li) await li.click();
  st = await page.$eval(pk, (w) => ({ text: w.querySelector('input[type=text]').value, id: w.querySelector('input[type=hidden]').value }));
  check('related asset: choosing stores its id and shows its label', st.id === '11' && st.text === 'Pump A', JSON.stringify(st));
  await page.screenshot({ path: `/tmp/asset_loc_${theme}.png`, clip: { x: 0, y: 0, width: 1100, height: 900 } });

  // Map & photos tab
  check('Map & photos tab found', await clickTab('Map & photos'), '');
  await sleep(400);
  const gps = '[data-seam-widget="gps"]';
  const compact = await page.$eval(gps, (w) => {
    const row = w.querySelector('.fe-picker-row').getBoundingClientRect(); const inp = w.querySelector('input[type=text]').getBoundingClientRect();
    const t = w.querySelector('[data-gps-toggle]'); const l = w.querySelector('[data-gps-locate]');
    return { rowW: Math.round(row.width), inputW: Math.round(inp.width), toggleW: Math.round(t.getBoundingClientRect().width), toggleText: t.textContent.trim(), locateText: l.textContent.trim(), locateTitle: l.title };
  });
  check('GPS: one small pin button, the input keeps the row', compact.toggleW <= 36 && compact.toggleText === 'place' && compact.inputW >= compact.rowW - 50, JSON.stringify(compact));
  check('GPS: "use my location" is an icon-only button with a tooltip', compact.locateText === 'my_location' && compact.locateTitle.length > 0, JSON.stringify(compact));
  await page.click(gps + ' [data-gps-toggle]'); await sleep(900);
  const map = await page.$eval(gps, (w) => { const m = w.querySelector('[data-gps-map]'); const pin = m.querySelector('.leaflet-marker-icon'); return { value: w.querySelector('input[type=text]').value, w: m.clientWidth, h: m.clientHeight, tiles: m.querySelectorAll('.leaflet-tile').length, svg: !!(pin && pin.querySelector('svg')), imgs: m.querySelectorAll('img.leaflet-marker-icon,img.leaflet-marker-shadow').length }; });
  check('GPS: panel opens on the stored point with tiles and an SVG pin', map.value === '-34.9011,-56.1645' && map.w > 0 && map.h > 0 && map.tiles >= 4 && map.svg && map.imgs === 0, JSON.stringify(map));
  const geo = await page.$eval(gps, (w) => { const p = w.querySelector('[data-gps-panel]'); const pr = p.getBoundingClientRect(), tr = w.querySelector('[data-gps-toggle]').getBoundingClientRect(); return { dRight: Math.round(pr.right - tr.right), below: pr.top >= tr.bottom - 1, pw: Math.round(pr.width) }; });
  check('GPS: the panel sits under the pin button, aligned to its right edge', geo.below && Math.abs(geo.dRight) <= 2 && geo.pw <= 400, JSON.stringify(geo));
  await page.click(gps + ' [data-gps-expand]'); await sleep(700);
  const wideM = await page.$eval(gps, (w) => { const p = w.querySelector('[data-gps-panel]'); return { pw: Math.round(p.getBoundingClientRect().width), rw: Math.round(w.getBoundingClientRect().width), tiles: w.querySelectorAll('.leaflet-tile').length }; });
  check('GPS: expand takes the full width of the field', Math.abs(wideM.pw - wideM.rw) <= 2 && wideM.tiles >= 4, JSON.stringify(wideM));
  await page.click(gps + ' [data-gps-expand]'); await sleep(400);
  const box = await (await page.$(gps + ' [data-gps-map]')).boundingBox();
  await page.mouse.click(box.x + box.width * 0.25, box.y + box.height * 0.25); await sleep(400);
  const moved = await page.$eval(gps + ' input[type=text]', (e) => e.value);
  check('GPS: clicking the map changes the coordinates', /^-?\d+\.\d{6},-?\d+\.\d{6}$/.test(moved) && moved !== '-34.9011,-56.1645', moved);
  await page.screenshot({ path: `/tmp/asset_geo_${theme}.png`, clip: { x: 0, y: 0, width: 1100, height: 1000 } });
  await page.click(gps + ' [data-gps-close]');
  const up = '[owner-type="asset"][owner-id="7"][owner-field="photos"]';
  await page.waitForFunction(() => !!customElements.get('seam-file-upload'), { timeout: 4000 }).catch(() => {});
  await sleep(600);
  const gal = await page.evaluate((sel) => { const el = document.querySelector('seam-file-upload' + sel); const r = el && el.getBoundingClientRect(); const lab = el && el.querySelector('.seam-dz-label'); return { present: !!el, defined: !!customElements.get('seam-file-upload'), h: r && Math.round(r.height), w: r && Math.round(r.width), label: lab && lab.textContent.trim(), icon: !!(el && el.querySelector('.seam-dz-icon')) }; }, up);
  check('photos: the uploader is visible, with this asset as owner and an Add photos action', gal.present && gal.defined && gal.h > 0 && gal.w > 0 && gal.label === 'Add photos' && gal.icon, JSON.stringify(gal));
  check('no failed requests and no page errors', problems.length === 0, [...new Set(problems)].join(' | ').slice(0, 400));
  await page.close();
}

(async () => {
  await new Promise((r) => server.listen(0, r));
  const base = 'http://127.0.0.1:' + server.address().port;
  const browser = await puppeteer.launch({ executablePath: process.env.CHROME_BIN, args: ['--no-sandbox'], headless: true });
  for (const theme of (process.env.THEMES || 'light,dark').split(',')) await runTheme(browser, base, theme).catch((e) => results.push([false, `[${theme}] aborted`, e.message]));
  await browser.close(); server.close();
  let failed = 0;
  for (const [ok, name, detail] of results) { if (!ok) failed++; console.log((ok ? 'PASS' : 'FAIL') + '  ' + name + (ok ? '' : '   <- ' + detail)); }
  console.log(`\n${results.length - failed}/${results.length} passed`);
  process.exit(failed ? 1 : 0);
})().catch((e) => { console.error('harness error:', e); process.exit(2); });
