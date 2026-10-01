// Development tool. Run from m74studio; the input is a local decoded session.
// npm install --no-save playwright-core
// node scripts/screenshots.cjs test-output/session.json
const { chromium } = require('playwright-core');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');

(async () => {
  const project = path.resolve(__dirname, '..');
  const input = process.argv[2];
  if (!input) throw new Error('Pass a decoded session.json from M74Studio.exe -input recording.log -json session.json');
  const session = JSON.parse(fs.readFileSync(input, 'utf8'));
  session.file = 'demo-m74can.log';
  if (session.meta.VIN) session.meta.VIN = 'Скрыт';
  const out = path.join(project, 'docs/images');
  fs.mkdirSync(out, { recursive: true });
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1540, height: 960 }, deviceScaleFactor: 1 });
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.exposeFunction('initialLog', () => session);
    await page.exposeFunction('openLog', () => session);
    await page.exposeFunction('saveExport', () => '');
    const read = name => fs.readFileSync(path.join(project, 'ui', name), 'utf8');
    await page.setContent(read('index.html').replace('/*STYLE*/', () => read('style.css')).replace('/*SCRIPT*/', () => read('app.js')));
    await page.waitForFunction(() => document.querySelectorAll('.chart-canvas').length === 4);
    const chart = await page.locator('.chart-canvas').first().boundingBox();
    await page.mouse.move(chart.x + chart.width * .61, chart.y + chart.height * .45);
    await page.waitForTimeout(150);
    await page.screenshot({ path: path.join(out, 'overview.png') });
    await page.locator('#preset').selectOption('mixture');
    await page.mouse.move(chart.x + chart.width * .5, chart.y + chart.height * .45);
    await page.mouse.down();
    await page.mouse.move(chart.x + chart.width * .77, chart.y + chart.height * .45, { steps: 12 });
    await page.mouse.up();
    await page.waitForTimeout(150);
    await page.screenshot({ path: path.join(out, 'mixture.png') });
    await page.locator('[data-view="table"]').click();
    await page.screenshot({ path: path.join(out, 'table.png') });
    await page.locator('[data-view="graphs"]').click();
    await page.locator('#txtBtn').click();
    await page.screenshot({ path: path.join(out, 'export.png') });
    assert.deepEqual(errors, []);
    console.log('Saved four real UI screenshots to docs/images. No JavaScript errors.');
  } finally { await browser.close(); }
})().catch(e => { console.error(e); process.exit(1); });
