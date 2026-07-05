const { serve } = require('@hono/node-server');
const { Hono } = require('hono');

const app = new Hono();
const port = Number(process.env.PORT || 8080);

app.get('/', (c) => c.text('Hello from nimbopacks — node/hono sample\n'));

app.get('/healthz', (c) => c.json({ status: 'ok' }));

serve({ fetch: app.fetch, hostname: '0.0.0.0', port }, () => {
  console.log(`listening on :${port}`);
});
