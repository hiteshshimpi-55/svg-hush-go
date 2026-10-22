import assert from 'node:assert/strict';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

const modulePath = process.env.SVG_HUSH_PLAYWRIGHT;
const { chromium } = await import(modulePath ? pathToFileURL(modulePath).href : 'playwright');
let input = '';
for await (const chunk of process.stdin) input += chunk;
const fixtures = JSON.parse(input);

const server = http.createServer((_request, response) => {
  response.setHeader('Content-Type', 'text/html; charset=utf-8');
  response.end('<!doctype html><title>SVG-Hush browser regression</title>');
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
let browser;
try {
  browser = await chromium.launch({
    headless: true,
    ...(process.env.SVG_HUSH_CHROMIUM ? { executablePath: process.env.SVG_HUSH_CHROMIUM } : {}),
  });
  const page = await browser.newPage();
  const requests = [];
  await page.route('**/*', async (route) => {
    const url = route.request().url();
    if (url === origin + '/') return route.continue();
    requests.push(url);
    return route.abort();
  });
  await page.goto(origin);

  async function inspect(svg) {
    return page.evaluate(async (svg) => {
      window.__svgHushExecuted = 0;
      const frame = document.createElement('iframe');
      const objectURL = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
      frame.src = objectURL;
      const loaded = new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('SVG frame did not load')), 5000);
        frame.onload = () => { clearTimeout(timeout); resolve(); };
      });
      document.body.append(frame);
      await loaded;
      // Permit SMIL, load/error events, and resource fetches to become observable.
      await new Promise((resolve) => setTimeout(resolve, 100));
      const svgDocument = frame.contentDocument;
      svgDocument.querySelector('a')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      const result = {
        executions: window.__svgHushExecuted,
        rectangle: !!svgDocument.querySelector('rect'),
        scripts: svgDocument.querySelectorAll('script, foreignObject').length,
      };
      frame.remove();
      URL.revokeObjectURL(objectURL);
      return result;
    }, svg);
  }

  const control = await inspect(fixtures.positive);
  assert(control.executions > 0, 'positive control must execute script in the blob frame');
  requests.length = 0;
  for (const [index, svg] of fixtures.cases.entries()) {
    const result = await inspect(svg);
    assert.equal(result.executions, 0, `sanitized case ${index} executed script`);
    assert.equal(result.scripts, 0, `sanitized case ${index} retained active elements`);
    assert.equal(result.rectangle, true, `sanitized case ${index} lost its benign rectangle`);
  }
  assert.deepEqual(requests, [], 'fragment-only previews must not request external resources');
  console.log(`Chromium: positive control executed; ${fixtures.cases.length} sanitized cases retained shapes without script execution or resource requests.`);
} finally {
  await browser?.close();
  await new Promise((resolve) => server.close(resolve));
}
