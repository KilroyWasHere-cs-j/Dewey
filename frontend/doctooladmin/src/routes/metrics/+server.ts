import type { RequestHandler } from './$types';

const PROM_URL = 'http://localhost:9090/api/v1/query';

export const GET: RequestHandler = async ({ url }) => {
  const query = url.searchParams.get('query');

  if (!query) {
    return new Response(JSON.stringify({ error: 'Missing query' }), {
      status: 400
    });
  }

  try {
    const res = await fetch(`${PROM_URL}?query=${encodeURIComponent(query)}`);
    const data = await res.json();

    return new Response(JSON.stringify(data), {
      headers: { 'Content-Type': 'application/json' }
    });
  } catch (err) {
    return new Response(JSON.stringify({ error: 'Prometheus fetch failed' }), {
      status: 500
    });
  }
};