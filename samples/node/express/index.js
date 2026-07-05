const express = require('express');

const app = express();
const port = process.env.PORT || 8080;

app.get('/', (_req, res) => {
  res.type('text/plain').send('Hello from nimbopacks — node/express sample\n');
});

app.get('/healthz', (_req, res) => {
  res.json({ status: 'ok' });
});

app.listen(port, () => {
  console.log(`listening on :${port}`);
});
