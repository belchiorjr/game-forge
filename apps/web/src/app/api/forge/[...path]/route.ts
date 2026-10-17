// Same-origin proxy to the Go API: the bearer token stays server-side and SSE streams through untouched.
// Runs on the default Node runtime and is never cached because it reads the request.

const FORWARD_REQUEST_HEADERS = ["content-type", "idempotency-key", "last-event-id"];
const FORWARD_RESPONSE_HEADERS = ["content-type", "cache-control", "x-accel-buffering"];

async function proxy(req: Request, ctx: RouteContext<"/api/forge/[...path]">) {
  const { path } = await ctx.params;
  const base = process.env.FORGE_API_URL ?? "http://localhost:8787";
  const search = new URL(req.url).search;

  const headers = new Headers();
  for (const h of FORWARD_REQUEST_HEADERS) {
    const v = req.headers.get(h);
    if (v) headers.set(h, v);
  }
  if (process.env.FORGE_API_TOKEN) headers.set("authorization", `Bearer ${process.env.FORGE_API_TOKEN}`);

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  let upstream: Response;
  try {
    upstream = await fetch(`${base}/api/v1/${path.map(encodeURIComponent).join("/")}${search}`, {
      method: req.method,
      headers,
      body: hasBody ? await req.arrayBuffer() : undefined,
      signal: req.signal,
      cache: "no-store",
    });
  } catch {
    return Response.json(
      {
        error: {
          code: "api_unreachable",
          message: `API do Game Forge fora do ar em ${base}. Rode make api (ou make dev).`,
          details: {},
        },
      },
      { status: 502 },
    );
  }

  // Something else answered on FORGE_API_URL (a common clash when another stack uses the port).
  if (!upstream.headers.get("x-forge-api")) {
    await upstream.body?.cancel();
    return Response.json(
      {
        error: {
          code: "wrong_upstream",
          message: `${base} respondeu, mas não é a API do Game Forge. Confira FORGE_API_URL e se a API está rodando (make api).`,
          details: {},
        },
      },
      { status: 502 },
    );
  }

  const out = new Headers();
  for (const h of FORWARD_RESPONSE_HEADERS) {
    const v = upstream.headers.get(h);
    if (v) out.set(h, v);
  }
  return new Response(upstream.body, { status: upstream.status, headers: out });
}

export { proxy as GET, proxy as HEAD, proxy as POST, proxy as PUT, proxy as DELETE };
