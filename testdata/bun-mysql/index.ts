import { SQL } from "bun";

const sql = new SQL({
  url: process.env.DATABASE_URL,
  tls: { rejectUnauthorized: false },
});

await sql`
  CREATE TABLE IF NOT EXISTS visits (
    id INT AUTO_INCREMENT PRIMARY KEY,
    path TEXT NOT NULL,
    visited_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
  )
`;

const server = Bun.serve({
  port: process.env.PORT || 3000,

  async fetch(req) {
    const url = new URL(req.url);

    if (url.pathname === "/") {
      const path = "/";
      await sql`INSERT INTO visits (path) VALUES (${path})`;

      const rows = await sql`SELECT COUNT(*) as count FROM visits`;
      const count = rows[0].count;

      return new Response(`Hello! This page has been visited ${count} times.\n`);
    }

    if (url.pathname === "/visits") {
      const rows = await sql`
        SELECT path, visited_at FROM visits ORDER BY visited_at DESC LIMIT 20
      `;

      return Response.json(rows);
    }

    if (url.pathname === "/health") {
      try {
        await sql`SELECT 1`;
        return new Response("ok\n");
      } catch {
        return new Response("database unavailable\n", { status: 503 });
      }
    }

    return new Response("not found\n", { status: 404 });
  },
});

console.log(`Listening on http://localhost:${server.port}`);
