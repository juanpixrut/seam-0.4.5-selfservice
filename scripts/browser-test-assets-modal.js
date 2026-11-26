// End-to-end browser test of the Assets form as users get it: opened in the real SeamModal by
// htmx.ajax (as the Assets list does), served by the real handlers over a stateful fake xolu, with
// real clicks on Create, Create and add photos, Cancel and the Map & photos tab. It checks that
// the buttons do what they say, that the photo uploader works inside the modal (SeamModal rewrites
// the form's <script> elements), and that closing the modal refreshes the list when the asset list
// has changed. Fails on any 4xx/5xx (except map tiles) and any page error.
//
//   SEAM_ASSET_SERVE_FILE=$PWD/build/serve.txt go test ./internal/ui -run TestAssetForm_Serve -timeout 200s &
//   (wait for build/serve.txt)  PUPPETEER_CORE=<path to puppeteer-core> CHROME_BIN=<chromium> \
//     node scripts/browser-test-assets-modal.js build/serve.txt ; touch build/serve.txt.stop
//
// STATIC_ROOT overrides web/static. Dormant guard: listed in docs/KNOWN_ISSUES.md.
if (!process.env.CHROME_BIN) { console.error('set CHROME_BIN to a Chromium executable'); process.exit(2); }
const puppeteer = require(process.env.PUPPETEER_CORE || 'puppeteer-core');
const http = require('http'), fs = require('fs'), path = require('path'), os = require('os');
const ROOT = process.env.STATIC_ROOT || path.join(__dirname, '..', 'web', 'static');
const [SEAM, COOKIE, FAKE] = fs.readFileSync(process.argv[2] || 'build/serve.txt', 'utf8').trim().split('\n');
const MIME = { '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png', '.json': 'application/json', '.woff2': 'font/woff2', '.svg': 'image/svg+xml' };
const SHELL = `<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/css/tailwind.css"><link rel="stylesheet" href="/css/app.css"><link rel="stylesheet" href="/vendor/material-icons.css"><script src="/vendor/htmx@1.9.10.min.js"></script><script src="/js/app.js" defer></script></head><body class="bg-gray-50 dark:bg-gray-900"><div class="p-6"><h1>Assets</h1>
<button id="new" onclick="SeamModal.open('New asset');htmx.ajax('GET','/assets/dynamic/new',{target:'#seam-modal-body',swap:'innerHTML'})">New</button>
<button id="edit1" onclick="SeamModal.open('Edit asset');htmx.ajax('GET','/assets/dynamic/1/edit',{target:'#seam-modal-body',swap:'innerHTML'})">Edit 1</button></div></body></html>`;

function proxy(req, res) {
  const t = new URL(SEAM);
  const p = http.request({ host: t.hostname, port: t.port, path: req.url, method: req.method, headers: { ...req.headers, host: t.host, cookie: COOKIE } },
    (r) => { res.writeHead(r.statusCode, r.headers); r.pipe(res); });
  p.on('error', () => { res.statusCode = 502; res.end('bad gateway'); });
  req.pipe(p);
}
const server = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://x');
  if (u.pathname === '/shell') { res.setHeader('content-type', 'text/html'); return res.end(SHELL); }
  for (const rel of [u.pathname.replace(/^\/static/, ''), u.pathname]) {
    const f = path.join(ROOT, rel);
    if (rel !== '/' && f.startsWith(ROOT) && fs.existsSync(f) && fs.statSync(f).isFile()) {
      res.setHeader('content-type', MIME[path.extname(f)] || 'application/octet-stream');
      // A real network is not instant: Dropzone arrives after the uploader asked for it, as it does for a
      // user on a fresh page (SLOW_DROPZONE_MS=0 turns this off).
      if (/dropzone@/.test(u.pathname) && f.endsWith('.js')) { const body = fs.readFileSync(f); return setTimeout(() => res.end(body), +(process.env.SLOW_DROPZONE_MS || 600)); }
      return res.end(fs.readFileSync(f));
    }
  }
  proxy(req, res);
});

const results = [];
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const state = async () => JSON.parse(await (await fetch(FAKE + '/__state')).text()) || [];
const reset = () => fetch(FAKE + '/__reset', { method: 'POST' });

async function run(browser, base, theme) {
  const check = (name, ok, detail) => results.push([ok, `[${theme}] ${name}`, detail]);
  async function newPage() {
    const page = await browser.newPage();
    await page.setViewport({ width: 1300, height: 900 });
    page.problems = []; page.posts = [];
    page.on('response', (r) => { if (r.status() >= 400 && !/openstreetmap|vendor\/images\/search\.svg/.test(r.url())) page.problems.push(r.status() + ' ' + r.url().replace(base, '')); });
    page.on('pageerror', (e) => page.problems.push('pageerror: ' + e.message));
    page.on('request', (r) => { if (r.method() === 'POST') page.posts.push(r.url().replace(base, '')); });
    await page.goto(base + '/shell', { waitUntil: 'load' });
    if (theme === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
    return page;
  }
  const openModal = async (page, button) => {
    await page.click(button);
    await page.waitForSelector('#seam-modal-body form', { timeout: 8000 });
    await sleep(900);
  };
  const clickTab = (page, re) => page.evaluate((src) => { const b = [...document.querySelectorAll('#seam-modal-body button')].find((e) => new RegExp(src).test(e.textContent.trim())); if (b) b.click(); return !!b; }, re);
  const footer = async (page, label) => {
    const h = await page.evaluateHandle((label) => [...document.querySelectorAll('#seam-modal-footer button')].find((b) => b.textContent.trim() === label), label);
    const el = h.asElement();
    if (!el) throw new Error('no footer button "' + label + '"');
    await el.click();
  };
  const fillMain = async (page, name) => {
    await page.focus('#field-name'); await page.keyboard.type(name);
    await page.select('#field-type_id', '1'); await sleep(900);
    const crit = await page.$eval('#field-criticality', (s) => { const o = [...s.options].find((x) => x.value); return o && o.value; });
    await page.select('#field-criticality', crit);
  };
  const openPanelId = (page) => page.evaluate(() => { const p = [...document.querySelectorAll('#seam-modal-body .fe-tab-panel')].find((e) => getComputedStyle(e).display !== 'none'); return p && p.id; });
  const settle = async (page, ms) => { await Promise.race([page.waitForNavigation({ timeout: ms }).catch(() => {}), sleep(ms)]); await sleep(300); };

  // 1. Create from the first tab
  await reset();
  let page = await newPage();
  await openModal(page, '#new');
  await fillMain(page, 'Pump 1');
  await footer(page, 'Create');
  await settle(page, 4000);
  let st = await state();
  check('Create (Main tab, form filled): the asset is created and the list is shown', st.length === 1 && st[0].name === 'Pump 1' && /\/assets$/.test(new URL(page.url()).pathname), `created=${st.length} url=${page.url().replace(base, '')} posts=${page.posts}`);
  check('Create (Main tab): no failed requests or page errors', page.problems.length === 0, [...new Set(page.problems)].join(' | ').slice(0, 300));
  await page.close();

  // 2. Create while another tab is showing (the user went to Map & photos, then pressed Create)
  await reset();
  page = await newPage();
  await openModal(page, '#new');
  await fillMain(page, 'Pump 2');
  await clickTab(page, '^Map & photos$'); await sleep(400);
  await footer(page, 'Create');
  await settle(page, 4000);
  st = await state();
  check('Create from the Map & photos tab (form filled): the asset is created and the list is shown', st.length === 1 && st[0].name === 'Pump 2' && /\/assets$/.test(new URL(page.url()).pathname), `created=${st.length} url=${page.url().replace(base, '')} posts=${page.posts}`);
  await page.close();

  // 3. Create with required fields empty, from another tab: the user must be told, and shown where
  await reset();
  page = await newPage();
  await openModal(page, '#new');
  await clickTab(page, '^Map & photos$'); await sleep(400);
  await footer(page, 'Create');
  await settle(page, 2500);
  const fb = await page.evaluate(() => {
    const body = document.getElementById('seam-modal-body');
    const vis = (e) => { const r = e.getBoundingClientRect(); return r.width > 0 && r.height > 0 && getComputedStyle(e).visibility !== 'hidden'; };
    const red = [...body.querySelectorAll('*')].filter((e) => e.children.length === 0 && e.textContent.trim() && /red/.test(e.className) && vis(e)).map((e) => e.textContent.trim());
    const panel = [...body.querySelectorAll('.fe-tab-panel')].find((e) => getComputedStyle(e).display !== 'none');
    return { modalOpen: !!body.querySelector('form'), visibleErrors: red, panel: panel && panel.id, active: document.activeElement && document.activeElement.name };
  });
  st = await state();
  check('Create with required fields empty: nothing is created, the modal stays open', st.length === 0 && fb.modalOpen, JSON.stringify(fb));
  check('Create with required fields empty, from another tab: the form opens the tab with the first missing field and focuses it', fb.panel === 'tab-principal' && fb.active === 'name' && page.posts.length === 0, JSON.stringify({ ...fb, posts: page.posts }));
  await page.close();

  // 4. Create and add photos, then Cancel: the modal stays on the new asset's photos tab, and closing it shows the new asset in the list
  await reset();
  page = await newPage();
  await openModal(page, '#new');
  await fillMain(page, 'Pump 4');
  await footer(page, 'Create and add photos');
  await page.waitForFunction(() => { const f = document.querySelector('#seam-modal-body form'); return f && /\/assets\/dynamic\/1$/.test(f.getAttribute('action') || f.getAttribute('hx-post') || ''); }, { timeout: 6000 }).catch(() => {});
  await sleep(1200);
  const ph = await page.evaluate(() => {
    const body = document.getElementById('seam-modal-body');
    const up = body.querySelector('seam-file-upload');
    const r = up && up.getBoundingClientRect();
    const g = document.getElementById('tab-geo');
    return { editForm: !!body.querySelector('form'), geoShown: !!g && getComputedStyle(g).display !== 'none', defined: !!customElements.get('seam-file-upload'), uploaderOwner: up && up.getAttribute('owner-id'), w: r && Math.round(r.width), h: r && Math.round(r.height), label: (up && up.querySelector('.seam-dz-label') || {}).textContent };
  });
  st = await state();
  check('Create and add photos: the asset is created and the modal shows its Map & photos tab', st.length === 1 && ph.editForm && ph.geoShown && ph.uploaderOwner === '1', JSON.stringify(ph));
  check('Create and add photos: the photo uploader works inside the modal (defined, visible, "Add photos")', ph.defined && ph.w > 0 && ph.h > 0 && /Add photos/.test(ph.label || ''), JSON.stringify(ph));
  await page.evaluate(() => { window.__noReload = true; });
  await footer(page, 'Cancel');
  await settle(page, 3000);
  const reloaded = await page.evaluate(() => window.__noReload === undefined);
  check('Cancel after Create and add photos: the page behind the modal is reloaded', reloaded, `url=${page.url().replace(base, '')}`);
  const listHtml = await (await fetch(base + '/assets')).text();
  check('the list page shows the asset that Create and add photos created', /Pump 4/.test(listHtml), listHtml.replace(/\s+/g, ' ').slice(0, 120));
  await page.close();

  // 5. Editing an existing asset: the uploader works in the modal
  page = await newPage();
  await openModal(page, '#edit1');
  await clickTab(page, '^Map & photos$'); await sleep(700);
  const ed = await page.evaluate(() => { const up = document.querySelector('#seam-modal-body seam-file-upload'); const r = up && up.getBoundingClientRect(); return { defined: !!customElements.get('seam-file-upload'), w: r && Math.round(r.width), h: r && Math.round(r.height), label: (up && up.querySelector('.seam-dz-label') || {}).textContent }; });
  check('Edit: the photo uploader works inside the modal', ed.defined && ed.w > 0 && ed.h > 0 && /Add photos/.test(ed.label || ''), JSON.stringify(ed));
  await page.close();

  // 6. Photos, the way a user does it: first open of the form on a fresh page, upload, look, enlarge, come back
  await reset();
  await fetch(FAKE + '/api/v1/assets', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ name: 'Pump 6', type_id: 1, criticality: 'High' }) });
  page = await newPage();
  await openModal(page, '#edit1');
  await clickTab(page, '^Map & photos$');
  await sleep(2200);
  const uploaderState = () => page.evaluate(() => {
    const body = document.getElementById('seam-modal-body');
    const dz = body.querySelector('.seam-dropzone');
    const err = body.querySelector('.seam-error');
    return { error: err ? err.textContent.trim() : '', dropzone: !!(dz && dz.dropzoneInstance), thumbs: body.querySelectorAll('.seam-gallery-item img').length };
  });
  let us = await uploaderState();
  check('Photos: first open of the form on a fresh page is ready to upload (no "Dropzone did not load")', us.error === '' && us.dropzone, JSON.stringify(us));
  const titles = await page.evaluate(() => {
    const panel = document.getElementById('tab-geo');
    const vis = (e) => { const r = e.getBoundingClientRect(); return r.width > 2 && r.height > 2; };
    return [...panel.querySelectorAll('*')].filter((e) => e.children.length === 0 && /^Photos$/i.test(e.textContent.trim()) && vis(e)).map((e) => e.tagName + '.' + e.className);
  });
  check('Photos: the section is titled once (the engine\'s section title, no second "Photos" label)', titles.length === 1, JSON.stringify(titles));
  // every part of the drop area opens the file chooser: the icon, the "Add photos" button, the hint, the empty area
  for (const [name, sel] of [['icon', '.seam-dz-icon'], ['"Add photos" button', '.seam-dz-label'], ['hint text', '.seam-dz-hint'], ['empty area', null]]) {
    const dzBox = await (await page.$('#seam-modal-body .seam-dropzone')).boundingBox();
    let x = dzBox.x + 40, y = dzBox.y + dzBox.height / 2;
    if (sel) { const b = await (await page.$('#seam-modal-body ' + sel)).boundingBox(); x = b.x + b.width / 2; y = b.y + b.height / 2; }
    let opened = false;
    const waiting = page.waitForFileChooser({ timeout: 1500 }).then((c) => { opened = true; return c.cancel(); }).catch(() => {});
    await page.mouse.click(x, y);
    await waiting;
    check(`Photos: clicking the ${name} opens the file chooser`, opened, name);
  }
  const png = path.join(os.tmpdir(), 'seam-e2e-photo.png');
  fs.writeFileSync(png, Buffer.from(await page.evaluate(() => { const c = document.createElement('canvas'); c.width = 640; c.height = 420; const x = c.getContext('2d'); x.fillStyle = '#4f46e5'; x.fillRect(0, 0, 640, 420); x.fillStyle = '#fff'; x.fillRect(40, 40, 200, 120); return c.toDataURL('image/png').split(',')[1]; }), 'base64'));
  const [chooser] = await Promise.all([page.waitForFileChooser({ timeout: 5000 }), page.click('#seam-modal-body .seam-dropzone')]);
  await chooser.accept([png]);
  await page.waitForFunction(() => document.querySelectorAll('#seam-modal-body .seam-gallery-item img').length === 1 || document.querySelector('#seam-modal-body .seam-error'), { timeout: 8000 }).catch(() => {});
  await sleep(500);
  us = await uploaderState();
  const media = await (await fetch(FAKE + '/__media')).json();
  check('Photos: an uploaded photo stays on the form and is stored', us.thumbs === 1 && us.error === '' && media.length === 1 && media[0].owner_id === 1, JSON.stringify({ ...us, stored: media.length }));
  const cur = await page.$eval('#seam-modal-body .seam-gallery-item img', (i) => getComputedStyle(i).cursor);
  check('Photos: the thumbnail shows a hand cursor', cur === 'pointer', cur);
  if (process.env.SHOT_DIR) await page.screenshot({ path: path.join(process.env.SHOT_DIR, `photos-tab-${theme}.png`) });
  await page.click('#seam-modal-body .seam-gallery-item img');
  await sleep(600);
  if (process.env.SHOT_DIR) await page.screenshot({ path: path.join(process.env.SHOT_DIR, `photo-enlarged-${theme}.png`) });
  const lb = await page.evaluate(() => { const l = document.querySelector('.seam-lightbox'); const i = l && l.querySelector('img'); const r = i && i.getBoundingClientRect(); return { open: !!l, w: r && Math.round(r.width), nat: i && i.naturalWidth }; });
  check('Photos: clicking the thumbnail shows the photo larger, in front of the form', lb.open && lb.nat === 640 && lb.w > 300, JSON.stringify(lb));
  await page.keyboard.press('Escape');
  await sleep(400);
  const after = await page.evaluate(() => ({ lightbox: !!document.querySelector('.seam-lightbox'), formOpen: !!document.querySelector('#seam-modal-body form') }));
  check('Photos: Escape closes only the enlarged photo, not the asset form', !after.lightbox && after.formOpen, JSON.stringify(after));
  await page.close();
  page = await newPage();
  await openModal(page, '#edit1');
  await clickTab(page, '^Map & photos$');
  await sleep(2200);
  us = await uploaderState();
  check('Photos: reopening the same asset shows its photo and no error', us.thumbs === 1 && us.error === '' && us.dropzone, JSON.stringify(us));
  check('Photos: no failed requests or page errors', page.problems.length === 0, [...new Set(page.problems)].join(' | ').slice(0, 300));
  await page.close();
}

(async () => {
  await new Promise((r) => server.listen(0, r));
  const base = 'http://127.0.0.1:' + server.address().port;
  const browser = await puppeteer.launch({ executablePath: process.env.CHROME_BIN, args: ['--no-sandbox'], headless: true });
  for (const theme of (process.env.THEMES || 'light').split(',')) await run(browser, base, theme).catch((e) => results.push([false, `[${theme}] aborted`, e.message]));
  await browser.close(); server.close();
  let failed = 0;
  for (const [ok, name, detail] of results) { if (!ok) failed++; console.log((ok ? 'PASS' : 'FAIL') + '  ' + name + (ok ? '' : '   <- ' + detail)); }
  console.log(`\n${results.length - failed}/${results.length} passed`);
  process.exit(failed ? 1 : 0);
})().catch((e) => { console.error('harness error:', e); process.exit(2); });
