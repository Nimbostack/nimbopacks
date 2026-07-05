const Fastify = require('fastify');

const app = Fastify();
const port = process.env.PORT || 8080;

app.get('/', (_req, reply) => {
  reply.type('text/plain').send('Hello from nimbopacks — node/fastify sample\n');
});

app.get('/healthz', (_req, reply) => {
  reply.send({ status: 'ok' });
});

app.listen({ host: '0.0.0.0', port: Number(port) }, (err) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }
  console.log(`listening on :${port}`);
});
