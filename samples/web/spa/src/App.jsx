import { useState } from 'react';

export default function App() {
  const [count, setCount] = useState(0);
  return (
    <main style={{ fontFamily: 'system-ui, sans-serif', padding: '2rem' }}>
      <h1>Hello from nimbopacks — web/spa sample</h1>
      <p>Built with Vite + React, served by nginx with push-state routing.</p>
      <button onClick={() => setCount((c) => c + 1)}>count: {count}</button>
    </main>
  );
}
